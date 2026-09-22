package server

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/dev-kryptic/Kryptic.Encryption.Go/envelope"
	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/applog"
	"github.com/dev-kryptic/daemon/internal/dynsecret"
	"github.com/dev-kryptic/daemon/internal/dynsecret/provider"
	"github.com/dev-kryptic/daemon/internal/orgkey"
)

func (s *Server) handleSecrets(req request) any {
	cacheKey := req.ProjectId + "/" + req.Environment

	s.mu.Lock()
	entry, hit := s.cache[cacheKey]
	fresh := hit && time.Since(entry.fetchedAt) < cacheTTL
	s.mu.Unlock()

	token, err := s.token()
	if err != nil {
		applog.Error("daemon", "bundle.auth", err)
		return fail("not_authenticated", "run `kryptic login` first")
	}

	var secrets []orgkey.Pair
	var hints []api.DynamicHint
	var orgKey []byte
	if fresh {
		secrets = entry.secrets
		hints = entry.hints
		orgKey = entry.orgKey
	} else {
		bundle, err := s.client.Bundle(token, req.ProjectId, req.Environment)
		if err != nil {
			if apiError, ok := asAPIError(err); ok {
				applog.Error("daemon", "bundle.fetch", err, "status="+strconv.Itoa(apiError.Status))
				switch apiError.Status {
				case 401:
					return fail("not_authenticated", "session revoked - run `kryptic login`")
				case 403:
					message := "you have no access to this environment"
					if apiError.Message != "" {
						message = apiError.Message
					}
					if strings.Contains(strings.ToLower(message), "organization key") {
						notifyMissingOrgKey()
					} else {
						notifyProjectDenied(req.ProjectId, req.Environment, message)
					}
					return fail("access_denied", message)
				case 404:
					if isUnknownEnvironment(apiError) {
						return fail("unknown_environment", apiError.Message)
					}
					return fail("unknown_project", apiError.Message)
				}
			}
			return fail("internal", err.Error())
		}

		decrypted, key, err := orgkey.OpenDeviceBundle(bundle)
		if err != nil {
			log.Printf("bundle decrypt failed for %s/%s", req.ProjectId, req.Environment)
			applog.Event("daemon", "bundle.decrypt", "result=error")
			if isGrantFailure(err) {
				notifyMissingOrgKey()
				return fail("access_denied", err.Error())
			}
			return fail("internal", err.Error())
		}
		secrets = decrypted
		hints = bundle.DynamicSecrets
		orgKey = key
		s.mu.Lock()
		s.cache[cacheKey] = cacheEntry{secrets: secrets, hints: hints, orgKey: orgKey, fetchedAt: time.Now()}
		s.mu.Unlock()
	}

	if len(hints) > 0 && !fresh {
		leased, leaseIDs, err := s.dynamicPairs(token, req.ProjectId, req.Environment, hints, orgKey, entry.leaseIDs)
		if err != nil {
			return fail("internal", err.Error())
		}
		for _, pair := range leased {
			secrets = append(secrets, orgkey.Pair{Key: pair.Key, Value: pair.Value})
		}
		s.mu.Lock()
		cached := s.cache[cacheKey]
		cached.secrets = secrets
		cached.leaseIDs = leaseIDs
		s.cache[cacheKey] = cached
		s.mu.Unlock()
	}

	log.Printf("served %d secrets for %s/%s (decrypted locally)", len(secrets), req.ProjectId, req.Environment)
	applog.Event("daemon", "bundle.ok", "count="+strconv.Itoa(len(secrets)))
	return secretsReply{V: protocolVersion, OK: true, Secrets: secrets}
}

func (s *Server) dynamicPairs(token, projectId, environment string, hints []api.DynamicHint, orgKey []byte, existing []string) ([]dynsecret.Pair, []string, error) {
	if pairs, ok := reuseLeasePairs(s.client, token, orgKey, existing); ok {
		return pairs, existing, nil
	}
	result, err := dynsecret.Fetch(context.Background(), s.client, token, projectId, environment, hints, orgkey.Device, orgKey)
	if err != nil {
		return nil, nil, err
	}
	return result.Pairs, result.LeaseIDs, nil
}

func reuseLeasePairs(client *api.Client, token string, orgKey []byte, leaseIDs []string) ([]dynsecret.Pair, bool) {
	if len(leaseIDs) == 0 {
		return nil, false
	}
	var pairs []dynsecret.Pair
	for _, id := range leaseIDs {
		env, err := client.DynamicLeaseEnvelope(token, id)
		if err != nil {
			return nil, false
		}
		associated := envelope.SecretContext(env.DefinitionId, env.EnvironmentId)
		plaintext, err := envelope.Open(orgKey, env.Envelope, associated)
		if err != nil {
			return nil, false
		}
		var minted provider.Minted
		if err := json.Unmarshal(plaintext, &minted); err != nil {
			return nil, false
		}
		pairs = append(pairs, dynsecret.Expand(env.Key, minted)...)
	}
	return pairs, true
}

func isUnknownEnvironment(err *api.APIError) bool {
	if err.Code == "unknown_environment" {
		return true
	}
	return strings.Contains(strings.ToLower(err.Message), "environment")
}

func isGrantFailure(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unwrap the org key") ||
		strings.Contains(msg, "no device key") ||
		strings.Contains(msg, "invalid wrapped org key") ||
		strings.Contains(msg, "device key is corrupt")
}

func asAPIError(err error) (*api.APIError, bool) {
	e, ok := err.(*api.APIError)
	return e, ok
}
