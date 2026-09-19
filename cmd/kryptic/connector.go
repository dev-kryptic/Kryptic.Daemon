package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dev-kryptic/Kryptic.Encryption.Go/envelope"
	"github.com/dev-kryptic/Kryptic.Encryption.Go/kdf"
	"github.com/dev-kryptic/Kryptic.Encryption.Go/sealedbox"
	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/authstore"
	"github.com/dev-kryptic/daemon/internal/dynsecret"
)

func connector() error {
	if len(os.Args) < 3 || os.Args[2] != "run" {
		fmt.Println(`kryptic connector - reap leftover dynamic-secret revokes

  kryptic connector run [--name NAME]

Optional. App, CI, and the dashboard mint on the process that asked for the
lease. This loop only drops roles this identity created and then left behind
(crash, missed ci revoke). Uses the signed-in daemon session, or
KRYPTIC_CLIENT_ID / KRYPTIC_CLIENT_SECRET against the Secrets API.`)
		return nil
	}
	return connectorRun()
}

func connectorRun() error {
	name := "kryptic-connector"
	args := os.Args[3:]
	for i := 0; i < len(args); i++ {
		if args[i] == "--name" && i+1 < len(args) {
			i++
			name = args[i]
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if os.Getenv("KRYPTIC_CLIENT_ID") != "" && os.Getenv("KRYPTIC_CLIENT_SECRET") != "" {
		return runMachineConnector(ctx, name)
	}
	return runDaemonConnector(ctx, name)
}

func runDaemonConnector(ctx context.Context, name string) error {
	client := api.NewClient()
	return dynsecret.RunConnector(ctx, client, func() (string, error) {
		return platformAccessToken(client)
	}, name, unwrapDeviceOrgKey)
}

func runMachineConnector(ctx context.Context, name string) error {
	clientId := os.Getenv("KRYPTIC_CLIENT_ID")
	clientSecret := os.Getenv("KRYPTIC_CLIENT_SECRET")
	client := api.NewSecretsAPIClient()
	tokens, err := client.MachineToken(clientId, clientSecret)
	if err != nil {
		return fmt.Errorf("credential exchange failed: %w", err)
	}
	keys, err := client.MachineKeysMe(tokens.AccessToken)
	if err != nil {
		return fmt.Errorf("fetching machine keys: %w", err)
	}
	return dynsecret.RunConnector(ctx, client, func() (string, error) {
		next, err := client.MachineToken(clientId, clientSecret)
		if err != nil {
			return "", err
		}
		return next.AccessToken, nil
	}, name, func(orgKeyId, wrapped string) ([]byte, error) {
		return unwrapMachineOrgKey(clientSecret, keys, wrapped)
	})
}

func unwrapDeviceOrgKey(_, wrapped string) ([]byte, error) {
	session, err := authstore.LoadSession()
	if err != nil {
		return nil, err
	}
	enc := base64.RawURLEncoding
	private, err := enc.DecodeString(session.DevicePrivateKey)
	if err != nil {
		return nil, err
	}
	public, err := enc.DecodeString(session.DevicePublicKey)
	if err != nil {
		return nil, err
	}
	sealed, err := sealedbox.Parse(wrapped)
	if err != nil {
		return nil, err
	}
	return sealedbox.Open(sealedbox.KeyPair{Public: public, Private: private}, sealed)
}

func unwrapMachineOrgKey(clientSecret string, keys *api.MachineKeys, wrapped string) ([]byte, error) {
	enc := base64.RawURLEncoding
	salt, err := enc.DecodeString(keys.KdfSalt)
	if err != nil {
		return nil, err
	}
	secretKey, err := kdf.ForVersion(keys.KdfParametersVersion, clientSecret, salt)
	if err != nil {
		return nil, err
	}
	privateKey, err := envelope.Open(secretKey, keys.WrappedPrivateKey, nil)
	if err != nil {
		return nil, err
	}
	publicKey, err := enc.DecodeString(keys.PublicKey)
	if err != nil {
		return nil, err
	}
	sealed, err := sealedbox.Parse(wrapped)
	if err != nil {
		return nil, err
	}
	return sealedbox.Open(sealedbox.KeyPair{Public: publicKey, Private: privateKey}, sealed)
}
