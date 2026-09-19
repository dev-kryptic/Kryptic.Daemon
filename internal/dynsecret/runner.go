package dynsecret

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	dyn "github.com/dev-kryptic/daemon/dynsecret"
	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/applog"
)

// CleanupInterval is how often an optional connector reaps this identity's
// leftover renew/revoke work. Create is never on that list.
const CleanupInterval = 60 * time.Second

// RunConnector heartbeats and reaps leftover revoke/renew work for leases
// this daemon or machine created. It does not mint other people's creates.
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
	log.Printf("connector %s registered (%s); reaping leftover revokes every %s", connector.Name, connector.Id, CleanupInterval)
	applog.Event("connector", "register", "id="+connector.Id)

	ticker := time.NewTicker(CleanupInterval)
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
			if err := client.DynamicConnectorDisconnect(accessToken, connector.Id); err != nil {
				if unauthorized(err) {
					refresh()
					err = client.DynamicConnectorDisconnect(accessToken, connector.Id)
				}
				if err != nil {
					log.Printf("connector disconnect: %v", err)
				}
			}
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

// MintLease fetches work for one lease this process requested and mints or
// revokes it.
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
	return applyBatchItem(ctx, client, accessToken, orgKey, batch.OrgKeyId, batch.Items[0])
}

// RevokeLease queues a revoke on the control plane and drops the role.
func RevokeLease(ctx context.Context, client *api.Client, accessToken, leaseId string, orgKeyFn func(orgKeyId, wrapped string) ([]byte, error)) error {
	if _, err := client.RevokeDynamicLease(accessToken, leaseId); err != nil {
		return err
	}
	err := MintLease(ctx, client, accessToken, leaseId, orgKeyFn)
	if err == nil {
		return nil
	}
	// Pending leases close on the revoke POST; there is nothing to drop.
	if isNoWork(err) {
		return nil
	}
	return err
}

func isNoWork(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "no work ")
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
		if err := applyBatchItem(ctx, client, accessToken, orgKey, batch.OrgKeyId, item); err != nil {
			log.Printf("lease %s: %v", item.LeaseId, err)
		}
	}
	return nil
}

func applyBatchItem(ctx context.Context, client *api.Client, accessToken string, orgKey []byte, orgKeyId string, item dyn.WorkItem) error {
	return dyn.Apply(ctx, orgKey, orgKeyId, item, func(body dyn.CompleteLease) error {
		_, err := client.CompleteDynamicWork(accessToken, api.CompleteDynamicLease{
			LeaseId:        body.LeaseId,
			Ok:             body.Ok,
			Error:          body.Error,
			EntityId:       body.EntityId,
			MintedEnvelope: body.MintedEnvelope,
			ExpiresAt:      body.ExpiresAt,
		})
		return err
	})
}
