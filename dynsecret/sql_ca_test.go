package dynsecret

import (
	"os"
	"strings"
	"testing"
)

func TestWriteTempCACleansUpAndSkipsEmpty(t *testing.T) {
	path, cleanup, err := writeTempCA("")
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Fatalf("empty CA wrote %q", path)
	}
	cleanup()

	path, cleanup, err = writeTempCA("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n")
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("expected a temp path")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "BEGIN CERTIFICATE") {
		t.Fatalf("wrote %q", body)
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("cleanup left %s: %v", path, err)
	}
}

func TestPostgresDSNPinsVerifyFullAndTheCAPath(t *testing.T) {
	cfg := ProviderConfig{
		Host:     "db.example",
		Username: "admin",
		Password: "p",
		Database: "app",
		SSL:      true,
		CA:       "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n",
	}
	dsn, cleanup, err := postgresDSN(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if !strings.Contains(dsn, "sslmode=verify-full") {
		t.Fatalf("dsn %q", dsn)
	}
	if !strings.Contains(dsn, "sslrootcert=") {
		t.Fatalf("missing sslrootcert in %q", dsn)
	}
}
