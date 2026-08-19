// `kryptic ci` - the CI/CD side of the CLI. Runs headless with machine
// credentials (KRYPTIC_CLIENT_ID / KRYPTIC_CLIENT_SECRET) and performs every
// cryptographic step locally: the client secret unwraps the machine private
// key, the private key opens the sealed org key, and the org key decrypts the
// envelopes. The platform serves ciphertext only.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/dev-kryptic/Kryptic.Encryption.Go/envelope"
	"github.com/dev-kryptic/Kryptic.Encryption.Go/kdf"
	"github.com/dev-kryptic/Kryptic.Encryption.Go/sealedbox"
	"github.com/dev-kryptic/daemon/internal/api"
)

func ci() error {
	if len(os.Args) < 3 || os.Args[2] != "export" {
		fmt.Println(`kryptic ci - pipeline secrets (machine credentials)

  kryptic ci export --project proj_x --env production [--format dotenv|shell|json]

Credentials come from the environment:
  KRYPTIC_CLIENT_ID       machine identity client id (kmi_...)
  KRYPTIC_CLIENT_SECRET   the client secret from the management console
  KRYPTIC_PIPELINES_API   optional API override (self-hosted)`)
		return nil
	}
	return ciExport()
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

	pairs, err := decryptMachineBundle(clientSecret, keys, bundle)
	if err != nil {
		return err
	}

	switch format {
	case "dotenv":
		for _, pair := range pairs {
			fmt.Println(dotenvLine(pair.Key, pair.Value))
		}
	case "shell":
		for _, pair := range pairs {
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

type ciPair struct {
	Key   string
	Value string
}

// decryptMachineBundle runs the full local decryption chain:
// clientSecret -Argon2id-> unwrap machine private key -sealed box-> org key
// -AES-GCM-> plaintext values.
func decryptMachineBundle(clientSecret string, keys *api.MachineKeys, bundle *api.Bundle) ([]ciPair, error) {
	enc := base64.RawURLEncoding
	salt, err := enc.DecodeString(keys.KdfSalt)
	if err != nil {
		return nil, fmt.Errorf("invalid KDF salt encoding: %w", err)
	}
	secretKey, err := kdf.ForVersion(keys.KdfParametersVersion, clientSecret, salt)
	if err != nil {
		return nil, err
	}

	privateKey, err := envelope.Open(secretKey, keys.WrappedPrivateKey, nil)
	if err != nil {
		return nil, fmt.Errorf("could not unwrap the machine private key - wrong client secret?")
	}
	publicKey, err := enc.DecodeString(keys.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid machine public key encoding: %w", err)
	}

	sealed, err := sealedbox.Parse(bundle.WrappedOrgKey)
	if err != nil {
		return nil, fmt.Errorf("invalid wrapped org key: %w", err)
	}
	orgKey, err := sealedbox.Open(sealedbox.KeyPair{Public: publicKey, Private: privateKey}, sealed)
	if err != nil {
		return nil, fmt.Errorf("could not unwrap the org key - the machine grant may be stale, rotate the identity")
	}

	pairs := make([]ciPair, 0, len(bundle.Secrets))
	for _, entry := range bundle.Secrets {
		associatedData := envelope.SecretContext(entry.DefinitionId, entry.EnvironmentId)
		plaintext, err := envelope.Open(orgKey, entry.Envelope, associatedData)
		if err != nil {
			return nil, fmt.Errorf("decrypt %q: %w", entry.Key, err)
		}
		pairs = append(pairs, ciPair{Key: entry.Key, Value: string(plaintext)})
	}
	return pairs, nil
}
