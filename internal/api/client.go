// Package api is the daemon's client for the Kryptic Daemon BFF: device-flow
// login, token refresh, and secret bundle fetches.
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dev-kryptic/daemon/internal/applog"
	"github.com/dev-kryptic/daemon/internal/authstore"
	"github.com/dev-kryptic/daemon/internal/config"
)

const DefaultBaseURL = config.DefaultAPI
const DefaultPipelinesBaseURL = "https://pipelines.kryptic.dev"
const DefaultSecretsBaseURL = "https://secrets.kryptic.dev"

type Client struct {
	BaseURL     string
	HTTP        *http.Client
	TokenPath   string
	KeysPath    string
	DynamicPath string
}

func NewClient() *Client {
	return NewClientFor(authstore.ResolvedAPI())
}

func NewClientFor(baseURL string) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.IdleConnTimeout = 30 * time.Second
	transport.MaxIdleConns = 4
	transport.MaxIdleConnsPerHost = 2
	return &Client{
		BaseURL:     baseURL,
		HTTP:        &http.Client{Timeout: 30 * time.Second, Transport: transport},
		TokenPath:   "/api/token",
		KeysPath:    "/api/keys/me",
		DynamicPath: "/api/dynamic-secrets",
	}
}

func NewSecretsAPIClient() *Client {
	base := os.Getenv("KRYPTIC_SECRETS_API")
	if base == "" {
		base = DefaultSecretsBaseURL
	}
	client := NewClientFor(base)
	client.TokenPath = "/v1/token"
	client.KeysPath = "/v1/keys/me"
	client.DynamicPath = "/v1/dynamic-secrets"
	return client
}

func NewPipelinesClient() *Client {
	base := os.Getenv("KRYPTIC_PIPELINES_API")
	if base == "" {
		base = DefaultPipelinesBaseURL
	}
	client := NewClientFor(base)
	client.HTTP.Timeout = 15 * time.Second
	return client
}

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
		applog.Error("kryptic", "http", err, "method="+request.Method, "path="+request.URL.Path)
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		apiErr := readError(response)
		applog.Error("kryptic", "http", apiErr, "method="+request.Method, "path="+request.URL.Path,
			"status="+strconv.Itoa(response.StatusCode))
		return apiErr
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(response.Body).Decode(out)
}
