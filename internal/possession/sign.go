// Package possession signs the device-flow start nonce with the durable
// P-256 device key so the server can prove the daemon still holds it.
package possession

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/big"
)

// SignChallenge returns a raw IEEE P1363 R||S signature (base64url) of SHA-256(challenge).
func SignChallenge(privateKeyBase64Url, challengeBase64Url string) (string, error) {
	privBytes, err := base64.RawURLEncoding.DecodeString(privateKeyBase64Url)
	if err != nil || len(privBytes) != 32 {
		return "", fmt.Errorf("invalid device private key")
	}
	challenge, err := base64.RawURLEncoding.DecodeString(challengeBase64Url)
	if err != nil {
		return "", fmt.Errorf("invalid possession challenge")
	}

	curve := elliptic.P256()
	d := new(big.Int).SetBytes(privBytes)
	x, y := curve.ScalarBaseMult(privBytes)
	priv := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{Curve: curve, X: x, Y: y},
		D:         d,
	}

	sum := sha256.Sum256(challenge)
	r, s, err := ecdsa.Sign(rand.Reader, priv, sum[:])
	if err != nil {
		return "", err
	}

	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return base64.RawURLEncoding.EncodeToString(sig), nil
}
