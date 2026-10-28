// Package machine exchanges KRYPTIC_CLIENT_ID / KRYPTIC_CLIENT_SECRET for
// a short-lived token and the machine's wrapped key record.
package machine

import (
	"fmt"
	"os"

	"github.com/dev-kryptic/daemon/internal/api"
)

type Session struct {
	AccessToken  string
	ClientSecret string
	Keys         *api.MachineKeys
}

func Credentials() (clientId, clientSecret string, err error) {
	clientId = os.Getenv("KRYPTIC_CLIENT_ID")
	clientSecret = os.Getenv("KRYPTIC_CLIENT_SECRET")
	if clientId == "" || clientSecret == "" {
		return "", "", fmt.Errorf("set KRYPTIC_CLIENT_ID and KRYPTIC_CLIENT_SECRET (from the management console)")
	}
	return clientId, clientSecret, nil
}

func HasCredentials() bool {
	return os.Getenv("KRYPTIC_CLIENT_ID") != "" && os.Getenv("KRYPTIC_CLIENT_SECRET") != ""
}

func Token(client *api.Client) (string, error) {
	clientId, clientSecret, err := Credentials()
	if err != nil {
		return "", err
	}
	tokens, err := client.MachineToken(clientId, clientSecret)
	if err != nil {
		return "", fmt.Errorf("credential exchange failed: %w", err)
	}
	return tokens.AccessToken, nil
}

func Authenticate(client *api.Client) (Session, error) {
	_, clientSecret, err := Credentials()
	if err != nil {
		return Session{}, err
	}
	accessToken, err := Token(client)
	if err != nil {
		return Session{}, err
	}
	keys, err := client.MachineKeysMe(accessToken)
	if err != nil {
		return Session{}, fmt.Errorf("fetching machine keys: %w", err)
	}
	return Session{
		AccessToken:  accessToken,
		ClientSecret: clientSecret,
		Keys:         keys,
	}, nil
}
