package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dev-kryptic/Kryptic.Encryption.Go/envelope"
)

// WorkItem is one lease the requester must mint, renew, or revoke.
type WorkItem struct {
	LeaseId             string `json:"leaseId"`
	DefinitionId        string `json:"definitionId"`
	EnvironmentId       string `json:"environmentId"`
	Key                 string `json:"key"`
	Provider            int    `json:"provider"`
	Action              int    `json:"action"`
	UsernameTemplate    string `json:"usernameTemplate"`
	CreationStatement   string `json:"creationStatement"`
	RevocationStatement string `json:"revocationStatement"`
	RenewStatement      string `json:"renewStatement"`
	UrlTemplate         string `json:"urlTemplate"`
	ProviderEnvelope    string `json:"providerEnvelope"`
	IdentityName        string `json:"identityName"`
	DynamicSecretName   string `json:"dynamicSecretName"`
	ExpiresAt           string `json:"expiresAt"`
	EntityId            string `json:"entityId"`
}

// CompleteLease is posted back after Apply finishes (or fails).
type CompleteLease struct {
	LeaseId        string  `json:"leaseId"`
	Ok             bool    `json:"ok"`
	Error          string  `json:"error,omitempty"`
	EntityId       string  `json:"entityId,omitempty"`
	MintedEnvelope string  `json:"mintedEnvelope,omitempty"`
	ExpiresAt      *string `json:"expiresAt,omitempty"`
}

// Completer records the outcome on the control plane. Apply does not send
// the minted password in the clear; the envelope is sealed under orgKey.
type Completer func(CompleteLease) error

// Apply decrypts provider config, talks to the target, and completes the
// lease. The caller must already own this work item.
func Apply(ctx context.Context, orgKey []byte, orgKeyId string, item WorkItem, complete Completer) error {
	err := apply(ctx, orgKey, orgKeyId, item, complete)
	if err != nil {
		_ = complete(CompleteLease{LeaseId: item.LeaseId, Ok: false, Error: err.Error()})
	}
	return err
}

func apply(ctx context.Context, orgKey []byte, orgKeyId string, item WorkItem, complete Completer) error {
	associated := envelope.SecretContext(item.DefinitionId, item.EnvironmentId)
	plaintext, err := envelope.Open(orgKey, item.ProviderEnvelope, associated)
	if err != nil {
		return fmt.Errorf("decrypt provider config: %w", err)
	}
	cfg, err := ParseProviderConfig(plaintext)
	if err != nil {
		return err
	}
	provider, err := Lookup(item.Provider)
	if err != nil {
		return err
	}

	username := item.EntityId
	if username == "" {
		username = RenderUsername(item.UsernameTemplate, item.IdentityName, item.DynamicSecretName, ProviderName(item.Provider))
	}
	password := RandomString(32)
	expiresAt, _ := time.Parse(time.RFC3339, item.ExpiresAt)
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(time.Hour)
	}
	work := WorkContext{
		Config:              cfg,
		Username:            username,
		Password:            password,
		CreationStatement:   item.CreationStatement,
		RevocationStatement: item.RevocationStatement,
		RenewStatement:      item.RenewStatement,
		URLTemplate:         item.UrlTemplate,
		ExpiresAt:           expiresAt,
		EntityId:            item.EntityId,
	}

	switch item.Action {
	case ActionCreate:
		minted, err := provider.Mint(ctx, work)
		if err != nil {
			return err
		}
		if minted.Username == "" {
			minted.Username = username
		}
		minted.URL = RenderURL(item.UrlTemplate, minted, cfg, expiresAt)
		payload, _ := json.Marshal(minted)
		sealed, err := envelope.Seal(orgKey, orgKeyId, payload, associated)
		if err != nil {
			return err
		}
		return complete(CompleteLease{
			LeaseId:        item.LeaseId,
			Ok:             true,
			EntityId:       minted.Username,
			MintedEnvelope: sealed,
			ExpiresAt:      rfc3339Ptr(expiresAt),
		})
	case ActionRenew:
		if err := provider.Renew(ctx, work); err != nil {
			return err
		}
		return complete(CompleteLease{
			LeaseId:   item.LeaseId,
			Ok:        true,
			EntityId:  username,
			ExpiresAt: rfc3339Ptr(expiresAt),
		})
	case ActionRevoke:
		if err := provider.Revoke(ctx, work); err != nil {
			return err
		}
		return complete(CompleteLease{
			LeaseId:  item.LeaseId,
			Ok:       true,
			EntityId: username,
		})
	default:
		return fmt.Errorf("unknown lease action %d", item.Action)
	}
}

// ProviderName is the username-template {{dynamicSecret.type}} value.
func ProviderName(provider int) string {
	switch provider {
	case ProviderPostgres:
		return "postgres"
	case ProviderMySQL:
		return "mysql"
	case ProviderOracle:
		return "oracle"
	case ProviderCassandra:
		return "cassandra"
	case ProviderRedis:
		return "redis"
	case ProviderAwsIAM:
		return "aws-iam"
	default:
		return "dynamic"
	}
}

func rfc3339Ptr(t time.Time) *string {
	s := t.UTC().Format(time.RFC3339)
	return &s
}
