// `kryptic ci` - the CI/CD side of the CLI. Runs headless with machine
// credentials (KRYPTIC_CLIENT_ID / KRYPTIC_CLIENT_SECRET) and performs every
// cryptographic step locally: the client secret unwraps the machine private
// key, the private key opens the sealed org key, and the org key decrypts the
// envelopes. The platform serves ciphertext only.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/dotenv"
	"github.com/dev-kryptic/daemon/internal/dynsecret"
	"github.com/dev-kryptic/daemon/internal/machine"
	"github.com/dev-kryptic/daemon/internal/orgkey"
)

func ci() error {
	if len(os.Args) < 3 {
		fmt.Println(`kryptic ci - pipeline secrets (machine credentials)

  kryptic ci export --project proj_x --env production [--format dotenv|shell|json]
  kryptic ci revoke [--lease ID]...

export mints dynamic leases on this runner. revoke drops those roles.
KRYPTIC_DYNAMIC_LEASES is written by export and read by revoke when
--lease is omitted.

Credentials come from the environment:
  KRYPTIC_CLIENT_ID       machine identity client id (kmi_...)
  KRYPTIC_CLIENT_SECRET   the client secret from the management console
  KRYPTIC_PIPELINES_API   optional API URL override`)
		return nil
	}
	switch os.Args[2] {
	case "export":
		return ciExport()
	case "revoke":
		return ciRevoke()
	default:
		return fmt.Errorf("unknown ci command %q (export or revoke)", os.Args[2])
	}
}

func ciExport() error {
	projectId, environment, format := "", "production", "dotenv"
	args := os.Args[3:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			i++
			projectId = args[i]
		case "--env":
			i++
			environment = args[i]
		case "--format":
			i++
			format = args[i]
		}
	}
	if projectId == "" {
		return fmt.Errorf("usage: kryptic ci export --project proj_x [--env production] [--format dotenv|shell|json]")
	}
	if format != "dotenv" && format != "shell" && format != "json" {
		return fmt.Errorf("unknown format %q - use dotenv, shell or json", format)
	}

	client := api.NewPipelinesClient()
	session, err := machine.Authenticate(client)
	if err != nil {
		return err
	}
	bundle, err := client.Bundle(session.AccessToken, projectId, environment)
	if err != nil {
		return fmt.Errorf("fetching secrets bundle: %w", err)
	}

	pairs, orgKey, err := orgkey.OpenMachineBundle(session.ClientSecret, session.Keys, bundle)
	if err != nil {
		return err
	}
	exported := make([]dotenv.Pair, 0, len(pairs))
	for _, pair := range pairs {
		exported = append(exported, dotenv.Pair{Key: pair.Key, Value: pair.Value})
	}
	if len(bundle.DynamicSecrets) > 0 {
		leased, err := dynsecret.Fetch(context.Background(), client, session.AccessToken, projectId, environment, bundle.DynamicSecrets, orgkey.Machine(session.ClientSecret, session.Keys), orgKey)
		if err != nil {
			return err
		}
		for _, pair := range leased.Pairs {
			exported = append(exported, dotenv.Pair{Key: pair.Key, Value: pair.Value})
		}
		if len(leased.LeaseIDs) > 0 {
			exported = append(exported, dotenv.Pair{Key: "KRYPTIC_DYNAMIC_LEASES", Value: strings.Join(leased.LeaseIDs, ",")})
		}
	}

	switch format {
	case "dotenv":
		dotenv.PrintDotenv(exported)
	case "shell":
		dotenv.PrintShell(exported)
	case "json":
		return dotenv.PrintJSON(exported)
	}
	return nil
}

func ciRevoke() error {
	var ids []string
	args := os.Args[3:]
	for i := 0; i < len(args); i++ {
		if args[i] == "--lease" && i+1 < len(args) {
			i++
			ids = append(ids, args[i])
		}
	}
	if len(ids) == 0 {
		for _, part := range strings.Split(os.Getenv("KRYPTIC_DYNAMIC_LEASES"), ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				ids = append(ids, part)
			}
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("set --lease ID or KRYPTIC_DYNAMIC_LEASES (written by kryptic ci export)")
	}

	client := api.NewPipelinesClient()
	session, err := machine.Authenticate(client)
	if err != nil {
		return err
	}
	unwrap := orgkey.Machine(session.ClientSecret, session.Keys)

	var failed int
	for _, id := range ids {
		if err := dynsecret.RevokeLease(context.Background(), client, session.AccessToken, id, unwrap); err != nil {
			fmt.Fprintf(os.Stderr, "revoke %s: %v\n", id, err)
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("revoked %d of %d leases", len(ids)-failed, len(ids))
	}
	return nil
}
