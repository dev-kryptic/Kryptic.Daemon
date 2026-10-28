package api

import "github.com/dev-kryptic/Kryptic.Encryption.Go/kdf"

type MachineKeys struct {
	PublicKey            string `json:"publicKey"`
	WrappedPrivateKey    string `json:"wrappedPrivateKey"`
	KdfSalt              string `json:"kdfSalt"`
	KdfParametersVersion int    `json:"kdfParametersVersion"`
}

func (c *Client) MachineToken(clientId, clientSecret string) (*Tokens, error) {
	authSecret, err := kdf.MachineAuthSecret(clientSecret)
	if err != nil {
		return nil, err
	}
	path := c.TokenPath
	if path == "" {
		path = "/api/token"
	}
	var out Tokens
	err = c.post(path, "", map[string]string{
		"clientId": clientId, "clientSecret": authSecret,
	}, &out)
	return &out, err
}

func (c *Client) MachineKeysMe(accessToken string) (*MachineKeys, error) {
	var out MachineKeys
	path := c.KeysPath
	if path == "" {
		path = "/api/keys/me"
	}
	err := c.get(path, accessToken, &out)
	return &out, err
}
