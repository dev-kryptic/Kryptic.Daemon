// Package orgkey unwraps the organization key and opens secret envelopes.
// Device sessions use the stored sealed-box key pair. Machine sessions
// derive the private key from the client secret. Both paths stay local.
package orgkey

import (
	"encoding/base64"
	"fmt"

	"github.com/dev-kryptic/Kryptic.Encryption.Go/envelope"
	"github.com/dev-kryptic/Kryptic.Encryption.Go/kdf"
	"github.com/dev-kryptic/Kryptic.Encryption.Go/sealedbox"
	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/authstore"
)

// Device opens a wrapped org key with the active profile's device keys.
// The first argument is unused; the signature matches lease orgKeyFn.
func Device(_, wrapped string) ([]byte, error) {
	return unwrapDevice(wrapped)
}

func unwrapDevice(wrapped string) ([]byte, error) {
	public, private, err := deviceKeys()
	if err != nil {
		return nil, err
	}
	return openWrapped(public, private, wrapped, "could not unwrap the org key - the device grant may be stale, run `kryptic login` again")
}

func deviceKeys() (public, private []byte, err error) {
	session, err := authstore.LoadSession()
	if err != nil {
		return nil, nil, err
	}
	if session.DevicePrivateKey == "" {
		return nil, nil, fmt.Errorf("this session has no device key - run `kryptic login` again")
	}
	enc := base64.RawURLEncoding
	private, err = enc.DecodeString(session.DevicePrivateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("stored device key is corrupt - run `kryptic login` again")
	}
	public, err = enc.DecodeString(session.DevicePublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("stored device key is corrupt - run `kryptic login` again")
	}
	return public, private, nil
}

// Machine returns an orgKeyFn that opens wrapped org keys with a machine
// identity's private key (unwrapped from the client secret).
func Machine(clientSecret string, keys *api.MachineKeys) func(orgKeyId, wrapped string) ([]byte, error) {
	return func(_, wrapped string) ([]byte, error) {
		return unwrapMachine(clientSecret, keys, wrapped)
	}
}

func unwrapMachine(clientSecret string, keys *api.MachineKeys, wrapped string) ([]byte, error) {
	public, private, err := machineKeys(clientSecret, keys)
	if err != nil {
		return nil, err
	}
	return openWrapped(public, private, wrapped, "could not unwrap the org key - the machine grant may be stale, rotate the identity")
}

func machineKeys(clientSecret string, keys *api.MachineKeys) (public, private []byte, err error) {
	enc := base64.RawURLEncoding
	salt, err := enc.DecodeString(keys.KdfSalt)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid KDF salt encoding: %w", err)
	}
	secretKey, err := kdf.ForVersion(keys.KdfParametersVersion, clientSecret, salt)
	if err != nil {
		return nil, nil, err
	}
	private, err = envelope.Open(secretKey, keys.WrappedPrivateKey, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("could not unwrap the machine private key - wrong client secret?")
	}
	public, err = enc.DecodeString(keys.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid machine public key encoding: %w", err)
	}
	return public, private, nil
}

func openWrapped(public, private []byte, wrapped, failMessage string) ([]byte, error) {
	sealed, err := sealedbox.Parse(wrapped)
	if err != nil {
		return nil, fmt.Errorf("invalid wrapped org key: %w", err)
	}
	orgKey, err := sealedbox.Open(sealedbox.KeyPair{Public: public, Private: private}, sealed)
	if err != nil {
		return nil, fmt.Errorf("%s", failMessage)
	}
	return orgKey, nil
}
