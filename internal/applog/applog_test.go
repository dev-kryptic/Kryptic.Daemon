package applog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeStripsSecretsAndNames(t *testing.T) {
	in := "user owner@kryptic.dev bearer abcdef Authorization: xyz RefreshToken=supersecrettokenvalue0123456789"
	out := Sanitize(in)
	if strings.Contains(out, "@kryptic.dev") || strings.Contains(out, "owner@") {
		t.Fatalf("email survived: %q", out)
	}
	if strings.Contains(out, "abcdef") || strings.Contains(out, "supersecret") {
		t.Fatalf("token survived: %q", out)
	}
}

func TestRotateCapsFileSize(t *testing.T) {
	t.Setenv("KRYPTIC_CONFIG_DIR", t.TempDir())

	previous := maxFileBytes
	maxFileBytes = 200
	t.Cleanup(func() { maxFileBytes = previous })

	Event("daemon", "test.first", "k=v")
	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("n", 180)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	Event("daemon", "test.rotate", "k=v")

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("current log missing: %v", err)
	}
	backup := filepath.Join(filepath.Dir(path), backupName)
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("backup missing after rotate: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "test.rotate") {
		t.Fatalf("expected event in new log: %s", data)
	}
}

func TestEventDoesNotWriteEmail(t *testing.T) {
	t.Setenv("KRYPTIC_CONFIG_DIR", t.TempDir())
	Event("daemon", "auth.me", "email=owner@kryptic.dev")
	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "owner@kryptic.dev") {
		t.Fatalf("email written to log: %s", data)
	}
	if !strings.Contains(string(data), "[email]") {
		t.Fatalf("expected redacted email: %s", data)
	}
}
