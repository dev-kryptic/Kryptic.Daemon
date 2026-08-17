// Package envelope opens Kryptic secret envelopes: the versioned container
// every Kryptic ciphertext travels in ("v1.<keyId>.<nonce>.<ciphertext+tag>",
// base64url without padding, AES-256-GCM). Secret values are end-to-end
// encrypted under the org key with associated data binding the ciphertext to
// its secret definition + environment, so rows cannot be swapped undetected.
//
// The daemon only ever opens envelopes - sealing happens in the management
// browser (and, for machines, in the CI client).
package envelope

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const (
	formatVersion = "v1"
	nonceSize     = 12
	tagSize       = 16
)

// Envelope is a parsed Kryptic secret envelope.
type Envelope struct {
	KeyID             string
	Nonce             []byte
	CiphertextWithTag []byte
}

// Parse parses the canonical serialized form.
func Parse(value string) (Envelope, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 4 || parts[0] != formatVersion {
		return Envelope{}, errors.New("value is not a valid kryptic secret envelope")
	}

	enc := base64.RawURLEncoding
	nonce, err := enc.DecodeString(parts[2])
	if err != nil {
		return Envelope{}, fmt.Errorf("invalid envelope nonce encoding: %w", err)
	}
	ciphertext, err := enc.DecodeString(parts[3])
	if err != nil {
		return Envelope{}, fmt.Errorf("invalid envelope ciphertext encoding: %w", err)
	}
	if len(nonce) != nonceSize {
		return Envelope{}, errors.New("envelope nonce must be 12 bytes")
	}
	if len(ciphertext) < tagSize {
		return Envelope{}, errors.New("envelope ciphertext shorter than the authentication tag")
	}

	return Envelope{KeyID: parts[1], Nonce: nonce, CiphertextWithTag: ciphertext}, nil
}

// Open decrypts a serialized envelope with the given 256-bit key. The
// associated data must match what the encrypting client bound the ciphertext
// to (for secret values: "secret:<definitionId>:env:<environmentId>").
func Open(key []byte, serialized string, associatedData []byte) ([]byte, error) {
	parsed, err := Parse(serialized)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := gcm.Open(nil, parsed.Nonce, parsed.CiphertextWithTag, associatedData)
	if err != nil {
		return nil, errors.New("envelope authentication failed - wrong key or tampered ciphertext")
	}
	return plaintext, nil
}

// SecretContext builds the associated data binding a secret value envelope to
// its row, matching the browser and the C# engine byte for byte.
func SecretContext(definitionID, environmentID string) []byte {
	return []byte("secret:" + strings.ToLower(definitionID) + ":env:" + strings.ToLower(environmentID))
}
