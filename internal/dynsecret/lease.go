package dynsecret

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/dev-kryptic/Kryptic.Encryption.Go/envelope"
	dyn "github.com/dev-kryptic/daemon/dynsecret"
	"github.com/dev-kryptic/daemon/internal/api"
)

// Result is the injected pairs plus the lease ids this process must revoke
// when the job or daemon is done.
type Result struct {
	Pairs    []Pair
	LeaseIDs []string
}

// Fetch requests a lease for each hint, mints it on this process, and expands
// minted JSON into KEY_USERNAME / KEY_PASSWORD / optional extras. It first
// reaps leftover revoke work this identity still owns.
func Fetch(ctx context.Context, client *api.Client, accessToken, projectPublicId, environment string, hints []api.DynamicHint, orgKeyFn func(orgKeyId, wrapped string) ([]byte, error), orgKey []byte) (Result, error) {
	if err := processWork(ctx, client, accessToken, orgKeyFn); err != nil {
		log.Printf("dynamic secret leftover revoke: %v", err)
	}

	var result Result
	for _, hint := range hints {
		lease, err := client.RequestDynamicLease(accessToken, projectPublicId, environment, hint.Key, "")
		if err != nil {
			return Result{}, fmt.Errorf("%s: %w", hint.Key, err)
		}
		if err := MintLease(ctx, client, accessToken, lease.Id, orgKeyFn); err != nil {
			return Result{}, fmt.Errorf("%s: %w", hint.Key, err)
		}
		env, err := client.DynamicLeaseEnvelope(accessToken, lease.Id)
		if err != nil {
			return Result{}, fmt.Errorf("%s envelope: %w", hint.Key, err)
		}
		associated := envelope.SecretContext(env.DefinitionId, env.EnvironmentId)
		plaintext, err := envelope.Open(orgKey, env.Envelope, associated)
		if err != nil {
			return Result{}, fmt.Errorf("decrypt %s: %w", hint.Key, err)
		}
		var minted dyn.Minted
		if err := json.Unmarshal(plaintext, &minted); err != nil {
			return Result{}, fmt.Errorf("%s minted value is not JSON: %w", hint.Key, err)
		}
		result.Pairs = append(result.Pairs, Expand(hint.Key, minted)...)
		result.LeaseIDs = append(result.LeaseIDs, lease.Id)
	}
	return result, nil
}

type Pair struct {
	Key   string
	Value string
}

func Expand(key string, minted dyn.Minted) []Pair {
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
