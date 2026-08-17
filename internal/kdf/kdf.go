// Package kdf derives keys from low-entropy secrets with Argon2id, matching
// Kryptic.Encryption's Argon2Parameters byte for byte (locked by the interop
// vector in testdata/). The CI path uses it to turn a machine's client secret
// into the key that unwraps the machine private key.
package kdf

import (
	"fmt"

	"golang.org/x/crypto/argon2"
)

const (
	// SaltSize matches Argon2KeyDerivation.SaltSizeBytes.
	SaltSize = 16
	// KeySize matches Argon2KeyDerivation.DerivedKeySizeBytes.
	KeySize = 32
)

// Argon2idV1 is parameter set 1: 64 MiB, 3 passes, 4 lanes, 32-byte output.
func Argon2idV1(secret string, salt []byte) ([]byte, error) {
	if len(salt) != SaltSize {
		return nil, fmt.Errorf("argon2 salt must be %d bytes", SaltSize)
	}
	return argon2.IDKey([]byte(secret), salt, 3, 64*1024, 4, KeySize), nil
}

// ForVersion dispatches on the parameter set version stored with the record.
func ForVersion(version int, secret string, salt []byte) ([]byte, error) {
	if version != 1 {
		return nil, fmt.Errorf("unknown Argon2 parameter set version %d", version)
	}
	return Argon2idV1(secret, salt)
}
