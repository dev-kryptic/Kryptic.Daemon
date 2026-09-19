package dynsecret

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/dev-kryptic/Kryptic.Encryption.Go/envelope"
	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/applog"
)

// RunConnector registers, heartbeats, and mints or revokes due leases.
// tokenFn is called at start and again after a 401 so a long-running
// connector survives access-token expiry and a Daemon BFF restart.
func RunConnector(ctx context.Context, client *api.Client, tokenFn func() (string, error), name string, orgKeyFn func(orgKeyId, wrapped string) ([]byte, error)) error {
	accessToken, err := tokenFn()
	if err != nil {
		return err
	}
	refresh := func() {
		next, refreshErr := tokenFn()
		if refreshErr != nil {
			log.Printf("connector token refresh: %v", refreshErr)
			return
		}
		accessToken = next
	}

	connector, err := client.RegisterDynamicConnector(accessToken, name)
	if err != nil {
		return fmt.Errorf("register connector: %w", err)
	}
	log.Printf("connector %s registered (%s)", connector.Name, connector.Id)
	applog.Event("connector", "register", "id="+connector.Id)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	if err := processWork(ctx, client, accessToken, orgKeyFn); err != nil {
		if unauthorized(err) {
			refresh()
		}
		log.Printf("connector work: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-heartbeat.C:
			if err := client.DynamicConnectorHeartbeat(accessToken, connector.Id); err != nil {
				if unauthorized(err) {
					refresh()
				}
				log.Printf("connector heartbeat: %v", err)
			}
		case <-ticker.C:
			if err := processWork(ctx, client, accessToken, orgKeyFn); err != nil {
				if unauthorized(err) {
					refresh()
				}
				log.Printf("connector work: %v", err)
			}
		}
	}
}

func unauthorized(err error) bool {
	var apiErr *api.APIError
	return errors.As(err, &apiErr) && apiErr.Status == 401
}

// MintLease fetches work for one dashboard lease and mints or revokes it.
func MintLease(ctx context.Context, client *api.Client, accessToken, leaseId string, orgKeyFn func(orgKeyId, wrapped string) ([]byte, error)) error {
	batch, err := client.DynamicLeaseWork(accessToken, leaseId)
	if err != nil {
		return err
	}
	if len(batch.Items) == 0 {
		return fmt.Errorf("no work for lease %s", leaseId)
	}
	orgKey, err := orgKeyFn(batch.OrgKeyId, batch.WrappedOrgKey)
	if err != nil {
		return err
	}
	item := batch.Items[0]
	if err := handleItem(ctx, client, accessToken, orgKey, batch.OrgKeyId, item); err != nil {
		log.Printf("lease %s: %v", item.LeaseId, err)
		_, _ = client.CompleteDynamicWork(accessToken, api.CompleteDynamicLease{
			LeaseId: item.LeaseId,
			Ok:      false,
			Error:   err.Error(),
		})
		return err
	}
	return nil
}

func processWork(ctx context.Context, client *api.Client, accessToken string, orgKeyFn func(orgKeyId, wrapped string) ([]byte, error)) error {
	batch, err := client.DynamicWork(accessToken)
	if err != nil {
		return err
	}
	if len(batch.Items) == 0 {
		return nil
	}
	orgKey, err := orgKeyFn(batch.OrgKeyId, batch.WrappedOrgKey)
	if err != nil {
		return err
	}
	for _, item := range batch.Items {
		if err := handleItem(ctx, client, accessToken, orgKey, batch.OrgKeyId, item); err != nil {
			log.Printf("lease %s: %v", item.LeaseId, err)
			_, _ = client.CompleteDynamicWork(accessToken, api.CompleteDynamicLease{
				LeaseId: item.LeaseId,
				Ok:      false,
				Error:   err.Error(),
			})
		}
	}
	return nil
}

func handleItem(ctx context.Context, client *api.Client, accessToken string, orgKey []byte, orgKeyId string, item api.DynamicWorkItem) error {
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
		username = RenderUsername(item.UsernameTemplate, item.IdentityName, item.DynamicSecretName, providerName(item.Provider))
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
		_, err = client.CompleteDynamicWork(accessToken, api.CompleteDynamicLease{
			LeaseId:        item.LeaseId,
			Ok:             true,
			EntityId:       minted.Username,
			MintedEnvelope: sealed,
			ExpiresAt:      rfc3339Ptr(expiresAt),
		})
		return err
	case ActionRenew:
		if err := provider.Renew(ctx, work); err != nil {
			return err
		}
		_, err = client.CompleteDynamicWork(accessToken, api.CompleteDynamicLease{
			LeaseId:   item.LeaseId,
			Ok:        true,
			EntityId:  username,
			ExpiresAt: rfc3339Ptr(expiresAt),
		})
		return err
	case ActionRevoke:
		if err := provider.Revoke(ctx, work); err != nil {
			return err
		}
		_, err = client.CompleteDynamicWork(accessToken, api.CompleteDynamicLease{
			LeaseId:  item.LeaseId,
			Ok:       true,
			EntityId: username,
		})
		return err
	default:
		return fmt.Errorf("unknown lease action %d", item.Action)
	}
}

func providerName(provider int) string {
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
