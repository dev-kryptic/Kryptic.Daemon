// `kryptic ci` - the CI/CD side of the CLI. Runs headless with machine
// credentials (KRYPTIC_CLIENT_ID / KRYPTIC_CLIENT_SECRET) and performs every
// cryptographic step locally: the client secret unwraps the machine private
// key, the private key opens the sealed org key, and the org key decrypts the
// envelopes. The platform serves ciphertext only.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/dev-kryptic/Kryptic.Encryption.Go/envelope"
	"github.com/dev-kryptic/Kryptic.Encryption.Go/kdf"
	"github.com/dev-kryptic/Kryptic.Encryption.Go/sealedbox"
	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/dynsecret"
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

	clientId := os.Getenv("KRYPTIC_CLIENT_ID")
	clientSecret := os.Getenv("KRYPTIC_CLIENT_SECRET")
	if clientId == "" || clientSecret == "" {
		return fmt.Errorf("set KRYPTIC_CLIENT_ID and KRYPTIC_CLIENT_SECRET (from the management console)")
	}

	client := api.NewPipelinesClient()
	tokens, err := client.MachineToken(clientId, clientSecret)
	if err != nil {
		return fmt.Errorf("credential exchange failed: %w", err)
	}

	keys, err := client.MachineKeysMe(tokens.AccessToken)
	if err != nil {
		return fmt.Errorf("fetching machine keys: %w", err)
	}
	bundle, err := client.Bundle(tokens.AccessToken, projectId, environment)
	if err != nil {
		return fmt.Errorf("fetching secrets bundle: %w", err)
	}

	pairs, orgKey, err := decryptMachineBundle(clientSecret, keys, bundle)
	if err != nil {
		return err
	}
	unwrap := func(_, wrapped string) ([]byte, error) {
		return unwrapMachineOrgKey(clientSecret, keys, wrapped)
	}
	if len(bundle.DynamicSecrets) > 0 {
		leased, err := dynsecret.Fetch(context.Background(), client, tokens.AccessToken, projectId, environment, bundle.DynamicSecrets, unwrap, orgKey)
		if err != nil {
			return err
		}
		for _, pair := range leased.Pairs {
			pairs = append(pairs, ciPair{Key: pair.Key, Value: pair.Value})
		}
		if len(leased.LeaseIDs) > 0 {
			pairs = append(pairs, ciPair{Key: "KRYPTIC_DYNAMIC_LEASES", Value: strings.Join(leased.LeaseIDs, ",")})
		}
	}

	switch format {
	case "dotenv":
		for _, pair := range exportablePairs(pairs) {
			fmt.Println(dotenvLine(pair.Key, pair.Value))
		}
	case "shell":
		for _, pair := range exportablePairs(pairs) {
			fmt.Printf("export %s='%s'\n", pair.Key, strings.ReplaceAll(pair.Value, "'", `'\''`))
		}
	case "json":
		object := make(map[string]string, len(pairs))
		for _, pair := range pairs {
			object[pair.Key] = pair.Value
		}
		encoded, _ := json.MarshalIndent(object, "", "  ")
		fmt.Println(string(encoded))
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

	clientId := os.Getenv("KRYPTIC_CLIENT_ID")
	clientSecret := os.Getenv("KRYPTIC_CLIENT_SECRET")
	if clientId == "" || clientSecret == "" {
		return fmt.Errorf("set KRYPTIC_CLIENT_ID and KRYPTIC_CLIENT_SECRET (from the management console)")
	}

	client := api.NewPipelinesClient()
	tokens, err := client.MachineToken(clientId, clientSecret)
	if err != nil {
		return fmt.Errorf("credential exchange failed: %w", err)
	}
	keys, err := client.MachineKeysMe(tokens.AccessToken)
	if err != nil {
		return fmt.Errorf("fetching machine keys: %w", err)
	}
	unwrap := func(_, wrapped string) ([]byte, error) {
		return unwrapMachineOrgKey(clientSecret, keys, wrapped)
	}

	var failed int
	for _, id := range ids {
		if err := dynsecret.RevokeLease(context.Background(), client, tokens.AccessToken, id, unwrap); err != nil {
			fmt.Fprintf(os.Stderr, "revoke %s: %v\n", id, err)
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("revoked %d of %d leases", len(ids)-failed, len(ids))
	}
	return nil
}

type ciPair struct {
	Key   string
	Value string
}

// exportablePairs drops keys that are not valid shell identifiers (see
// envKeyPattern) before dotenv/shell rendering, warning on stderr. JSON
// export passes keys through unchanged.
func exportablePairs(pairs []ciPair) []ciPair {
	kept := make([]ciPair, 0, len(pairs))
	var skipped []string
	for _, pair := range pairs {
		if envKeyPattern.MatchString(pair.Key) {
			kept = append(kept, pair)
		} else {
			skipped = append(skipped, pair.Key)
		}
	}
	warnSkippedKeys(skipped)
	return kept
}

// decryptMachineBundle runs the full local decryption chain:
// clientSecret -Argon2id-> unwrap machine private key -sealed box-> org key
// -AES-GCM-> plaintext values.
func decryptMachineBundle(clientSecret string, keys *api.MachineKeys, bundle *api.Bundle) ([]ciPair, []byte, error) {
	enc := base64.RawURLEncoding
	salt, err := enc.DecodeString(keys.KdfSalt)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid KDF salt encoding: %w", err)
	}
	secretKey, err := kdf.ForVersion(keys.KdfParametersVersion, clientSecret, salt)
	if err != nil {
		return nil, nil, err
	}

	privateKey, err := envelope.Open(secretKey, keys.WrappedPrivateKey, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("could not unwrap the machine private key - wrong client secret?")
	}
	publicKey, err := enc.DecodeString(keys.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid machine public key encoding: %w", err)
	}

	sealed, err := sealedbox.Parse(bundle.WrappedOrgKey)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid wrapped org key: %w", err)
	}
	orgKey, err := sealedbox.Open(sealedbox.KeyPair{Public: publicKey, Private: privateKey}, sealed)
	if err != nil {
		return nil, nil, fmt.Errorf("could not unwrap the org key - the machine grant may be stale, rotate the identity")
	}

	pairs := make([]ciPair, 0, len(bundle.Secrets))
	for _, entry := range bundle.Secrets {
		associatedData := envelope.SecretContext(entry.DefinitionId, entry.EnvironmentId)
		plaintext, err := envelope.Open(orgKey, entry.Envelope, associatedData)
		if err != nil {
			return nil, nil, fmt.Errorf("decrypt %q: %w", entry.Key, err)
		}
		pairs = append(pairs, ciPair{Key: entry.Key, Value: string(plaintext)})
	}
	return pairs, orgKey, nil
}
