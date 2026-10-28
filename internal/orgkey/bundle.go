package orgkey

import (
	"fmt"

	"github.com/dev-kryptic/Kryptic.Encryption.Go/envelope"
	"github.com/dev-kryptic/daemon/internal/api"
)

// Pair is one decrypted secret. The socket protocol and CLI exports use
// the same shape.
type Pair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// OpenEntries decrypts every envelope in a bundle with an already-unwrapped
// org key. Associated data is rebuilt from the definition and environment ids.
func OpenEntries(orgKey []byte, entries []api.BundleEntry) ([]Pair, error) {
	pairs := make([]Pair, 0, len(entries))
	for _, entry := range entries {
		associated := envelope.SecretContext(entry.DefinitionId, entry.EnvironmentId)
		plaintext, err := envelope.Open(orgKey, entry.Envelope, associated)
		if err != nil {
			return nil, fmt.Errorf("decrypt %q: %w", entry.Key, err)
		}
		pairs = append(pairs, Pair{Key: entry.Key, Value: string(plaintext)})
	}
	return pairs, nil
}

// OpenDeviceBundle unwraps the org key with this device and decrypts the
// static envelopes. Dynamic leases are fetched separately.
func OpenDeviceBundle(bundle *api.Bundle) ([]Pair, []byte, error) {
	orgKey, err := unwrapDevice(bundle.WrappedOrgKey)
	if err != nil {
		return nil, nil, err
	}
	pairs, err := OpenEntries(orgKey, bundle.Secrets)
	if err != nil {
		return nil, nil, err
	}
	return pairs, orgKey, nil
}

// OpenMachineBundle is OpenDeviceBundle for a CI/machine identity.
func OpenMachineBundle(clientSecret string, keys *api.MachineKeys, bundle *api.Bundle) ([]Pair, []byte, error) {
	orgKey, err := unwrapMachine(clientSecret, keys, bundle.WrappedOrgKey)
	if err != nil {
		return nil, nil, err
	}
	pairs, err := OpenEntries(orgKey, bundle.Secrets)
	if err != nil {
		return nil, nil, err
	}
	return pairs, orgKey, nil
}
