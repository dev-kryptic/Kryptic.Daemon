// Package sealedbox implements the Kryptic P-256 ECDH sealed box: encrypt a
// value to a recipient's public key so only the holder of the matching private
// key can open it. This is the asymmetric layer the blind store uses to
// deliver the org key to the daemon; the wire format and derivation are
// locked by the interop vectors in testdata/ (canonical copy lives in the
// Kryptic.Encryption repository).
//
// Construction (ECIES): fresh ephemeral P-256 key pair per message, ECDH against
// the recipient public key, HKDF-SHA256 expanded to a 32-byte AES key and a
// 12-byte nonce (derived, not random - the per-message key makes it safe and the
// seal reproducible), then AES-256-GCM. No custom primitives.
package sealedbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const (
	// FormatVersion is the sealed-box wire format version this package produces.
	FormatVersion = 1

	// PublicKeySize is the uncompressed SEC1 P-256 point: 0x04 || X(32) || Y(32).
	PublicKeySize = 65

	// PrivateKeySize is the P-256 scalar, big-endian.
	PrivateKeySize = 32

	keySize   = 32 // AES-256
	nonceSize = 12 // GCM standard
	tagSize   = 16
)

var hkdfLabel = []byte("kryptic-sealed-box-v1")

// KeyPair is a P-256 key pair in the portable encodings shared with the C# and
// WebCrypto implementations.
type KeyPair struct {
	Public  []byte // 65-byte uncompressed SEC1 point
	Private []byte // 32-byte big-endian scalar
}

// SealedKey is a parsed sealed box.
type SealedKey struct {
	RecipientKeyID     string
	EphemeralPublicKey []byte
	Nonce              []byte
	CiphertextWithTag  []byte
}

// GenerateKeyPair creates a fresh P-256 key pair.
func GenerateKeyPair() (KeyPair, error) {
	private, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return KeyPair{}, fmt.Errorf("generate P-256 key: %w", err)
	}
	return KeyPair{
		Public:  private.PublicKey().Bytes(),
		Private: private.Bytes(),
	}, nil
}

// Seal encrypts plaintext to the recipient public key with a fresh ephemeral key.
func Seal(recipientPublicKey []byte, recipientKeyID string, plaintext []byte) (SealedKey, error) {
	ephemeral, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return SealedKey{}, fmt.Errorf("generate ephemeral key: %w", err)
	}
	return sealWithEphemeral(ephemeral, recipientPublicKey, recipientKeyID, plaintext)
}

// Open decrypts a sealed box with the recipient key pair.
func Open(recipient KeyPair, box SealedKey) ([]byte, error) {
	private, err := ecdh.P256().NewPrivateKey(recipient.Private)
	if err != nil {
		return nil, fmt.Errorf("invalid recipient private key: %w", err)
	}
	ephemeralPublic, err := ecdh.P256().NewPublicKey(box.EphemeralPublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid ephemeral public key: %w", err)
	}

	shared, err := private.ECDH(ephemeralPublic)
	if err != nil {
		return nil, fmt.Errorf("ECDH agreement: %w", err)
	}

	key, _ := deriveKeyAndNonce(shared, box.EphemeralPublicKey, recipient.Public)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plaintext, err := gcm.Open(nil, box.Nonce, box.CiphertextWithTag, nil)
	if err != nil {
		return nil, errors.New("sealed box authentication failed")
	}
	return plaintext, nil
}

func sealWithEphemeral(ephemeral *ecdh.PrivateKey, recipientPublicKey []byte, recipientKeyID string, plaintext []byte) (SealedKey, error) {
	if !isValidKeyID(recipientKeyID) {
		return SealedKey{}, errors.New("recipient key id must be non-empty and contain only [a-zA-Z0-9_-]")
	}
	recipient, err := ecdh.P256().NewPublicKey(recipientPublicKey)
	if err != nil {
		return SealedKey{}, fmt.Errorf("invalid recipient public key: %w", err)
	}

	shared, err := ephemeral.ECDH(recipient)
	if err != nil {
		return SealedKey{}, fmt.Errorf("ECDH agreement: %w", err)
	}

	ephemeralPublicKey := ephemeral.PublicKey().Bytes()
	key, nonce := deriveKeyAndNonce(shared, ephemeralPublicKey, recipientPublicKey)

	block, err := aes.NewCipher(key)
	if err != nil {
		return SealedKey{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return SealedKey{}, err
	}
	ciphertextWithTag := gcm.Seal(nil, nonce, plaintext, nil)

	return SealedKey{
		RecipientKeyID:     recipientKeyID,
		EphemeralPublicKey: ephemeralPublicKey,
		Nonce:              nonce,
		CiphertextWithTag:  ciphertextWithTag,
	}, nil
}

// deriveKeyAndNonce expands the ECDH shared secret (the x-coordinate) with
// HKDF-SHA256 into the AES key and nonce, bound to both public keys.
func deriveKeyAndNonce(shared, ephemeralPublicKey, recipientPublicKey []byte) (key, nonce []byte) {
	info := make([]byte, 0, len(hkdfLabel)+len(ephemeralPublicKey)+len(recipientPublicKey))
	info = append(info, hkdfLabel...)
	info = append(info, ephemeralPublicKey...)
	info = append(info, recipientPublicKey...)

	okm := hkdfSHA256(shared, nil, info, keySize+nonceSize)
	return okm[:keySize], okm[keySize:]
}

// hkdfSHA256 is RFC 5869 extract-and-expand with SHA-256. Implemented inline
// (HMAC composition only) to avoid pulling golang.org/x/crypto into the daemon.
func hkdfSHA256(ikm, salt, info []byte, length int) []byte {
	if salt == nil {
		salt = make([]byte, sha256.Size)
	}
	extract := hmac.New(sha256.New, salt)
	extract.Write(ikm)
	prk := extract.Sum(nil)

	var okm []byte
	var block []byte
	for counter := byte(1); len(okm) < length; counter++ {
		expand := hmac.New(sha256.New, prk)
		expand.Write(block)
		expand.Write(info)
		expand.Write([]byte{counter})
		block = expand.Sum(nil)
		okm = append(okm, block...)
	}
	return okm[:length]
}

// Serialize renders the canonical wire form:
// sbx.v1.<recipientKeyId>.<ephemeralPub>.<nonce>.<ciphertext+tag> (base64url, no padding).
func (s SealedKey) Serialize() string {
	enc := base64.RawURLEncoding
	return strings.Join([]string{
		fmt.Sprintf("sbx.v%d", FormatVersion),
		s.RecipientKeyID,
		enc.EncodeToString(s.EphemeralPublicKey),
		enc.EncodeToString(s.Nonce),
		enc.EncodeToString(s.CiphertextWithTag),
	}, ".")
}

// Parse parses the canonical wire form.
func Parse(value string) (SealedKey, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 6 || parts[0] != "sbx" || parts[1] != fmt.Sprintf("v%d", FormatVersion) {
		return SealedKey{}, errors.New("value is not a valid kryptic sealed box")
	}
	if !isValidKeyID(parts[2]) {
		return SealedKey{}, errors.New("invalid sealed box recipient key id")
	}

	enc := base64.RawURLEncoding
	ephemeralPub, err := enc.DecodeString(parts[3])
	if err != nil {
		return SealedKey{}, fmt.Errorf("invalid ephemeral public key encoding: %w", err)
	}
	nonce, err := enc.DecodeString(parts[4])
	if err != nil {
		return SealedKey{}, fmt.Errorf("invalid nonce encoding: %w", err)
	}
	ciphertext, err := enc.DecodeString(parts[5])
	if err != nil {
		return SealedKey{}, fmt.Errorf("invalid ciphertext encoding: %w", err)
	}

	if len(ephemeralPub) != PublicKeySize || ephemeralPub[0] != 0x04 {
		return SealedKey{}, errors.New("ephemeral public key must be a 65-byte uncompressed SEC1 point")
	}
	if len(nonce) != nonceSize {
		return SealedKey{}, errors.New("sealed box nonce must be 12 bytes")
	}
	if len(ciphertext) < tagSize {
		return SealedKey{}, errors.New("sealed box ciphertext shorter than the authentication tag")
	}

	return SealedKey{
		RecipientKeyID:     parts[2],
		EphemeralPublicKey: ephemeralPub,
		Nonce:              nonce,
		CiphertextWithTag:  ciphertext,
	}, nil
}

func isValidKeyID(keyID string) bool {
	if keyID == "" || len(keyID) > 64 {
		return false
	}
	for _, c := range keyID {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}
