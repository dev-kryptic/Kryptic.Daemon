// Package api is the daemon's client for the Kryptic Daemon BFF: device-flow
// login, token refresh, and secret bundle fetches.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// DefaultBaseURL points at the hosted Daemon BFF; KRYPTIC_API overrides it
// for local development.
const DefaultBaseURL = "https://daemon.kryptic.dev"

// DefaultPipelinesBaseURL points at the hosted Pipelines BFF (CI/CD);
// KRYPTIC_PIPELINES_API overrides it.
const DefaultPipelinesBaseURL = "https://pipelines.kryptic.dev"

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient() *Client {
	base := os.Getenv("KRYPTIC_API")
	if base == "" {
		base = DefaultBaseURL
	}
	return &Client{BaseURL: base, HTTP: &http.Client{Timeout: 15 * time.Second}}
}

// NewPipelinesClient targets the Pipelines BFF, which speaks machine tokens
// (client credentials) instead of the daemon's user device flow.
func NewPipelinesClient() *Client {
	base := os.Getenv("KRYPTIC_PIPELINES_API")
	if base == "" {
		base = DefaultPipelinesBaseURL
	}
	return &Client{BaseURL: base, HTTP: &http.Client{Timeout: 15 * time.Second}}
}

type DeviceStart struct {
	DeviceCode          string `json:"deviceCode"`
	UserCode            string `json:"userCode"`
	VerificationUrl     string `json:"verificationUrl"`
	ExpiresInSeconds    int    `json:"expiresInSeconds"`
	PollIntervalSeconds int    `json:"pollIntervalSeconds"`
}

type Tokens struct {
	AccessToken      string `json:"accessToken"`
	RefreshToken     string `json:"refreshToken"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
}

type Me struct {
	Email        string `json:"email"`
	DisplayName  string `json:"displayName"`
	Organization string `json:"organization"`
}

// BundleEntry is one ciphertext envelope. The daemon decrypts it locally with
// the org key; the definition/environment ids rebuild the associated data.
type BundleEntry struct {
	Key           string `json:"key"`
	Envelope      string `json:"envelope"`
	DefinitionId  string `json:"definitionId"`
	EnvironmentId string `json:"environmentId"`
}

// Bundle is the end-to-end encrypted response: envelopes plus the org key
// sealed to this device's public key. The server cannot open any of it.
type Bundle struct {
	ProjectPublicId string        `json:"projectPublicId"`
	Environment     string        `json:"environment"`
	OrgKeyId        string        `json:"orgKeyId"`
	WrappedOrgKey   string        `json:"wrappedOrgKey"`
	Secrets         []BundleEntry `json:"secrets"`
}

type Project struct {
	PublicId     string   `json:"publicId"`
	Name         string   `json:"name"`
	Environments []string `json:"environments"`
}

// APIError carries the HTTP status so callers can distinguish auth problems
// (revoked session) from access problems (no grant for the project).
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%d: %s", e.Status, e.Message)
}

// DeviceStart begins the device flow. devicePublicKey is the device's
// sealed-box public key (base64url 65-byte SEC1 point); the approving admin
// seals the org key to it so this device can decrypt bundles.
func (c *Client) DeviceStart(deviceName, platform, version, devicePublicKey string) (*DeviceStart, error) {
	var out DeviceStart
	err := c.post("/api/auth/device/start", "", map[string]string{
		"deviceName": deviceName, "platform": platform, "daemonVersion": version,
		"devicePublicKey": devicePublicKey,
	}, &out)
	return &out, err
}

// DevicePoll returns (nil, nil) while the approval is still pending.
func (c *Client) DevicePoll(deviceCode string) (*Tokens, error) {
	body, _ := json.Marshal(map[string]string{"deviceCode": deviceCode})
	response, err := c.HTTP.Post(c.BaseURL+"/api/auth/device/poll", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusAccepted {
		return nil, nil // pending
	}
	if response.StatusCode != http.StatusOK {
		return nil, readError(response)
	}
	var tokens Tokens
	return &tokens, json.NewDecoder(response.Body).Decode(&tokens)
}

func (c *Client) Refresh(refreshToken string) (*Tokens, error) {
	var out Tokens
	err := c.post("/api/auth/refresh", "", map[string]string{"refreshToken": refreshToken}, &out)
	return &out, err
}

func (c *Client) Logout(accessToken string) error {
	return c.post("/api/auth/logout", accessToken, map[string]string{}, nil)
}

func (c *Client) Me(accessToken string) (*Me, error) {
	var out Me
	err := c.get("/api/auth/me", accessToken, &out)
	return &out, err
}

func (c *Client) Bundle(accessToken, projectPublicId, environment string) (*Bundle, error) {
	var out Bundle
	path := fmt.Sprintf("/api/secrets/bundle?projectPublicId=%s&environment=%s", projectPublicId, environment)
	err := c.get(path, accessToken, &out)
	return &out, err
}

func (c *Client) Projects(accessToken string) ([]Project, error) {
	var out []Project
	err := c.get("/api/secrets/projects", accessToken, &out)
	return out, err
}

// ---------- machine (CI) endpoints, Pipelines BFF ----------

// MachineKeys is the machine's own key record: the private key arrives wrapped
// by an Argon2id key derived from the client secret, so only a caller holding
// the secret can use it.
type MachineKeys struct {
	PublicKey            string `json:"publicKey"`
	WrappedPrivateKey    string `json:"wrappedPrivateKey"`
	KdfSalt              string `json:"kdfSalt"`
	KdfParametersVersion int    `json:"kdfParametersVersion"`
}

// MachineToken exchanges client credentials for a short-lived machine token.
func (c *Client) MachineToken(clientId, clientSecret string) (*Tokens, error) {
	var out Tokens
	err := c.post("/api/token", "", map[string]string{
		"clientId": clientId, "clientSecret": clientSecret,
	}, &out)
	return &out, err
}

// MachineKeysMe fetches the machine's key record.
func (c *Client) MachineKeysMe(accessToken string) (*MachineKeys, error) {
	var out MachineKeys
	err := c.get("/api/keys/me", accessToken, &out)
	return &out, err
}

// ---------- plumbing ----------

func (c *Client) post(path, accessToken string, payload any, out any) error {
	body, _ := json.Marshal(payload)
	request, _ := http.NewRequest(http.MethodPost, c.BaseURL+path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}
	return c.do(request, out)
}

func (c *Client) get(path, accessToken string, out any) error {
	request, _ := http.NewRequest(http.MethodGet, c.BaseURL+path, nil)
	request.Header.Set("Authorization", "Bearer "+accessToken)
	return c.do(request, out)
}

func (c *Client) do(request *http.Request, out any) error {
	response, err := c.HTTP.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return readError(response)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(response.Body).Decode(out)
}

func readError(response *http.Response) error {
	var buffer bytes.Buffer
	_, _ = buffer.ReadFrom(response.Body)
	message := buffer.String()

	// The API returns either a bare string or an { message } object.
	var wrapped struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(buffer.Bytes(), &wrapped) == nil && wrapped.Message != "" {
		message = wrapped.Message
	}
	var bare string
	if json.Unmarshal(buffer.Bytes(), &bare) == nil && bare != "" {
		message = bare
	}

	return &APIError{Status: response.StatusCode, Message: message}
}
