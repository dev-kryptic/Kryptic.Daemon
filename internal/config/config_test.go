package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeAPI(t *testing.T) {
	got, err := NormalizeAPI(" https://daemon.example.com/ ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://daemon.example.com" {
		t.Fatalf("got %q", got)
	}

	if _, err := NormalizeAPI("not a url"); err == nil {
		t.Fatal("expected error for missing scheme")
	}
	if _, err := NormalizeAPI("ftp://daemon.example.com"); err == nil {
		t.Fatal("expected error for ftp")
	}
	if _, err := NormalizeAPI(""); err == nil {
		t.Fatal("expected error for empty")
	}
}

func TestAPIResolution(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KRYPTIC_CONFIG_DIR", dir)
	t.Setenv("KRYPTIC_API", "")

	url, source := API()
	if url != DefaultAPI || source != SourceDefault {
		t.Fatalf("default: got %s (%s)", url, source)
	}

	if err := SetAPI("https://self-hosted.example:8443/"); err != nil {
		t.Fatal(err)
	}
	url, source = API()
	if url != "https://self-hosted.example:8443" || source != SourceFile {
		t.Fatalf("file: got %s (%s)", url, source)
	}

	t.Setenv("KRYPTIC_API", "http://localhost:5237")
	url, source = API()
	if url != "http://localhost:5237" || source != SourceEnvironment {
		t.Fatalf("env: got %s (%s)", url, source)
	}

	t.Setenv("KRYPTIC_API", "")
	if err := ResetAPI(); err != nil {
		t.Fatal(err)
	}
	url, source = API()
	if url != DefaultAPI || source != SourceDefault {
		t.Fatalf("reset: got %s (%s)", url, source)
	}

	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Fatal("expected config.json to remain after reset")
	}
}
