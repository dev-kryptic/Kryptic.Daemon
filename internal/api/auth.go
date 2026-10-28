package api

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type DeviceStart struct {
	DeviceCode          string `json:"deviceCode"`
	UserCode            string `json:"userCode"`
	VerificationUrl     string `json:"verificationUrl"`
	ExpiresInSeconds    int    `json:"expiresInSeconds"`
	PollIntervalSeconds int    `json:"pollIntervalSeconds"`
	Challenge           string `json:"challenge"`
}

type Tokens struct {
	AccessToken      string `json:"accessToken"`
	RefreshToken     string `json:"refreshToken"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
}

type Me struct {
	Email          string `json:"email"`
	DisplayName    string `json:"displayName"`
	Organization   string `json:"organization"`
	HasOrgKeyGrant *bool  `json:"hasOrgKeyGrant"`
}

func (c *Client) DeviceStart(deviceName, platform, version, devicePublicKey string) (*DeviceStart, error) {
	var out DeviceStart
	err := c.post("/api/auth/device/start", "", map[string]string{
		"deviceName": deviceName, "platform": platform, "daemonVersion": version,
		"devicePublicKey": devicePublicKey,
	}, &out)
	return &out, err
}

func (c *Client) DevicePoll(deviceCode, challengeSignature string) (*Tokens, error) {
	payload := map[string]string{"deviceCode": deviceCode}
	if challengeSignature != "" {
		payload["challengeSignature"] = challengeSignature
	}
	body, _ := json.Marshal(payload)
	response, err := c.HTTP.Post(c.BaseURL+"/api/auth/device/poll", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusAccepted {
		return nil, nil
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

func (c *Client) RevokeDevice(accessToken string) error {
	return c.post("/api/auth/device/revoke", accessToken, map[string]string{}, nil)
}

func (c *Client) Me(accessToken string) (*Me, error) {
	var out Me
	err := c.get("/api/auth/me", accessToken, &out)
	return &out, err
}
