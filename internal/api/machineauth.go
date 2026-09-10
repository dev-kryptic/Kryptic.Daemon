package api

import (
	"crypto/sha256"
	"encoding/base64"
	"io"
	"strings"

	"golang.org/x/crypto/hkdf"
)

// machineSecretV2Prefix marks client secrets whose token-exchange value is
// domain-separated from the wrap-key input. Canonical implementation:
// Kryptic.Encryption.Go kdf.MachineAuthSecret (adopt it here once the daemon
// picks up a lib release that includes it); locked cross-language by the
// lib's interop-vectors/machine-auth.json.
const machineSecretV2Prefix = "ksm2_"

// machineAuthSecret returns the value presented at token exchange. For v2
// secrets it is base64url(HKDF-SHA256(secret, "kryptic.machine.auth.v2",
// "auth", 32)); legacy secrets pass through unchanged.
func machineAuthSecret(secret string) (string, error) {
	if !strings.HasPrefix(secret, machineSecretV2Prefix) {
		return secret, nil
	}
	reader := hkdf.New(sha256.New, []byte(secret), []byte("kryptic.machine.auth.v2"), []byte("auth"))
	derived := make([]byte, 32)
	if _, err := io.ReadFull(reader, derived); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(derived), nil
}
