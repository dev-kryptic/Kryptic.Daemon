package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/dynsecret"
	"github.com/dev-kryptic/daemon/internal/login"
	"github.com/dev-kryptic/daemon/internal/machine"
	"github.com/dev-kryptic/daemon/internal/orgkey"
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

	if machine.HasCredentials() {
		return runMachineConnector(ctx, name)
	}
	return runDaemonConnector(ctx, name)
}

func runDaemonConnector(ctx context.Context, name string) error {
	client := api.NewClient()
	return dynsecret.RunConnector(ctx, client, func() (string, error) {
		return login.AccessToken(client)
	}, name, orgkey.Device)
}

func runMachineConnector(ctx context.Context, name string) error {
	client := api.NewSecretsAPIClient()
	session, err := machine.Authenticate(client)
	if err != nil {
		return err
	}
	return dynsecret.RunConnector(ctx, client, func() (string, error) {
		return machine.Token(client)
	}, name, orgkey.Machine(session.ClientSecret, session.Keys))
}
