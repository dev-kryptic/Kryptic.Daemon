package dynsecret

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dev-kryptic/Kryptic.Encryption.Go/envelope"
	"github.com/dev-kryptic/daemon/internal/api"
)

const (
	leasePollInterval = time.Second
	leaseWait         = 90 * time.Second
)

// Fetch requests a lease for each hint, waits for the connector, and expands
// minted JSON into KEY_USERNAME / KEY_PASSWORD / optional extras.
func Fetch(client *api.Client, accessToken, projectPublicId, environment string, hints []api.DynamicHint, orgKey []byte) ([]Pair, error) {
	var pairs []Pair
	for _, hint := range hints {
		lease, err := waitForLease(client, accessToken, projectPublicId, environment, hint.Key)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", hint.Key, err)
		}
		env, err := client.DynamicLeaseEnvelope(accessToken, lease.Id)
		if err != nil {
			return nil, fmt.Errorf("%s envelope: %w", hint.Key, err)
		}
		associated := envelope.SecretContext(env.DefinitionId, env.EnvironmentId)
		plaintext, err := envelope.Open(orgKey, env.Envelope, associated)
		if err != nil {
			return nil, fmt.Errorf("decrypt %s: %w", hint.Key, err)
		}
		var minted Minted
		if err := json.Unmarshal(plaintext, &minted); err != nil {
			return nil, fmt.Errorf("%s minted value is not JSON: %w", hint.Key, err)
		}
		pairs = append(pairs, Expand(hint.Key, minted)...)
	}
	return pairs, nil
}

func waitForLease(client *api.Client, accessToken, projectPublicId, environment, key string) (*api.DynamicLease, error) {
	lease, err := client.RequestDynamicLease(accessToken, projectPublicId, environment, key, "")
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(leaseWait)
	for {
		switch lease.Status {
		case 1: // Active
			if lease.HasMintedEnvelope {
				return lease, nil
			}
		case 4: // Failed
			if lease.Error != "" {
				return nil, fmt.Errorf("%s", lease.Error)
			}
			return nil, fmt.Errorf("lease failed")
		case 2, 3: // Revoked, Expired
			return nil, fmt.Errorf("lease is no longer active")
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("no connector completed this lease in time (is `kryptic connector run` online?)")
		}
		time.Sleep(leasePollInterval)
		lease, err = client.GetDynamicLease(accessToken, lease.Id)
		if err != nil {
			return nil, err
		}
	}
}

type Pair struct {
	Key   string
	Value string
}

func Expand(key string, minted Minted) []Pair {
	pairs := []Pair{
		{Key: key + "_USERNAME", Value: minted.Username},
		{Key: key + "_PASSWORD", Value: minted.Password},
	}
	if minted.SessionToken != "" {
		pairs = append(pairs, Pair{Key: key + "_SESSION_TOKEN", Value: minted.SessionToken})
	}
	if minted.AccessKeyId != "" {
		pairs = append(pairs, Pair{Key: key + "_ACCESS_KEY_ID", Value: minted.AccessKeyId})
	}
	if strings.TrimSpace(minted.URL) != "" {
		pairs = append(pairs, Pair{Key: key, Value: minted.URL})
	}
	return pairs
}
