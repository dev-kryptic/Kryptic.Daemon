package api

import (
	"net/url"

	"github.com/dev-kryptic/daemon/internal/dynsecret/provider"
)

type DynamicHint struct {
	Key           string `json:"key"`
	DefinitionId  string `json:"definitionId"`
	EnvironmentId string `json:"environmentId"`
	Provider      int    `json:"provider"`
}

type DynamicLease struct {
	Id                string `json:"id"`
	EnvironmentId     string `json:"environmentId"`
	Status            int    `json:"status"`
	RequestedAction   int    `json:"requestedAction"`
	RequestedByName   string `json:"requestedByName"`
	ExpiresAt         string `json:"expiresAt"`
	CompletedAt       string `json:"completedAt"`
	EntityId          string `json:"entityId"`
	Error             string `json:"error"`
	HasMintedEnvelope bool   `json:"hasMintedEnvelope"`
}

type DynamicWorkBatch struct {
	OrgKeyId      string              `json:"orgKeyId"`
	WrappedOrgKey string              `json:"wrappedOrgKey"`
	Items         []provider.WorkItem `json:"items"`
}

type CompleteDynamicLease struct {
	LeaseId        string  `json:"leaseId"`
	Ok             bool    `json:"ok"`
	Error          string  `json:"error,omitempty"`
	EntityId       string  `json:"entityId,omitempty"`
	MintedEnvelope string  `json:"mintedEnvelope,omitempty"`
	ExpiresAt      *string `json:"expiresAt,omitempty"`
}

type DynamicConnector struct {
	Id         string `json:"id"`
	Name       string `json:"name"`
	LastSeenAt string `json:"lastSeenAt"`
	Online     bool   `json:"online"`
}

type SecretEnvelope struct {
	Key           string `json:"key"`
	DefinitionId  string `json:"definitionId"`
	EnvironmentId string `json:"environmentId"`
	Envelope      string `json:"envelope"`
	Version       int    `json:"version"`
}

func (c *Client) dynamicPath() string {
	if c.DynamicPath != "" {
		return c.DynamicPath
	}
	return "/api/dynamic-secrets"
}

func (c *Client) RequestDynamicLease(accessToken, projectPublicId, environment, key, ttl string) (*DynamicLease, error) {
	query := url.Values{"projectPublicId": {projectPublicId}, "environment": {environment}, "key": {key}}
	if ttl != "" {
		query.Set("ttl", ttl)
	}
	var out DynamicLease
	err := c.post(c.dynamicPath()+"/leases?"+query.Encode(), accessToken, map[string]string{}, &out)
	return &out, err
}

func (c *Client) GetDynamicLease(accessToken, leaseId string) (*DynamicLease, error) {
	var out DynamicLease
	err := c.get(c.dynamicPath()+"/leases/"+leaseId, accessToken, &out)
	return &out, err
}

func (c *Client) DynamicLeaseEnvelope(accessToken, leaseId string) (*SecretEnvelope, error) {
	var out SecretEnvelope
	err := c.get(c.dynamicPath()+"/leases/"+leaseId+"/envelope", accessToken, &out)
	return &out, err
}

func (c *Client) RegisterDynamicConnector(accessToken, name string) (*DynamicConnector, error) {
	var out DynamicConnector
	err := c.post(c.dynamicPath()+"/connectors", accessToken, map[string]string{"name": name}, &out)
	return &out, err
}

func (c *Client) DynamicConnectorHeartbeat(accessToken, connectorId string) error {
	return c.post(c.dynamicPath()+"/connectors/"+connectorId+"/heartbeat", accessToken, map[string]string{}, nil)
}

func (c *Client) DynamicConnectorDisconnect(accessToken, connectorId string) error {
	return c.post(c.dynamicPath()+"/connectors/"+connectorId+"/disconnect", accessToken, map[string]string{}, nil)
}

func (c *Client) DynamicWork(accessToken string) (*DynamicWorkBatch, error) {
	var out DynamicWorkBatch
	err := c.get(c.dynamicPath()+"/work", accessToken, &out)
	return &out, err
}

func (c *Client) DynamicLeaseWork(accessToken, leaseId string) (*DynamicWorkBatch, error) {
	var out DynamicWorkBatch
	err := c.get(c.dynamicPath()+"/leases/"+leaseId+"/work", accessToken, &out)
	return &out, err
}

func (c *Client) RevokeDynamicLease(accessToken, leaseId string) (*DynamicLease, error) {
	var out DynamicLease
	err := c.post(c.dynamicPath()+"/leases/"+leaseId+"/revoke", accessToken, map[string]string{}, &out)
	return &out, err
}

func (c *Client) CompleteDynamicWork(accessToken string, body CompleteDynamicLease) (*DynamicLease, error) {
	var out DynamicLease
	err := c.post(c.dynamicPath()+"/work/complete", accessToken, body, &out)
	return &out, err
}
