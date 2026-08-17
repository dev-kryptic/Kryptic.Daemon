package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadConfig(t *testing.T) *Config {
	t.Helper()
	config, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return config
}

func TestLoadParsesFullRuleset(t *testing.T) {
	config := loadConfig(t)

	if len(config.Rules) != 222 {
		t.Fatalf("parsed %d rules, want 222", len(config.Rules))
	}
	for _, rule := range config.Rules {
		if rule.ID == "" {
			t.Fatal("rule without id")
		}
		if rule.Regex == nil && rule.Path == nil {
			t.Fatalf("rule %s has neither regex nor path", rule.ID)
		}
	}
	if len(config.Global.Paths) == 0 || len(config.Global.Regexes) == 0 {
		t.Fatal("global allowlist not parsed")
	}
}

func TestDetectsSeededSecrets(t *testing.T) {
	config := loadConfig(t)

	content := strings.Join([]string{
		`aws_secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"`,
		`github_token = "ghp_16C7e42F292c6912E7710c838347Ae178B4a"`,
		`-----BEGIN RSA PRIVATE KEY-----`,
		`MIIEpAIBAAKCAQEA7Zx5kml3Kx1qzD2ml3kFm29le5mlq2FyD2ml3kFm29le5mlq`,
		`2FyD2ml3kFm29le5mlq2FyD2ml3kFm29le5mlq2FyD2ml3kFm29le5mlq2FyAAAA`,
		`-----END RSA PRIVATE KEY-----`,
	}, "\n")

	findings := config.ScanContent("app/config.py", content)

	found := map[string]bool{}
	for _, finding := range findings {
		found[finding.RuleID] = true
	}
	if !found["github-pat"] {
		t.Errorf("github PAT not detected; findings: %+v", findings)
	}
	if !found["private-key"] {
		t.Errorf("private key not detected; findings: %+v", findings)
	}
}

func TestGlobalPathAllowlistSkipsLockfiles(t *testing.T) {
	config := loadConfig(t)

	directory := t.TempDir()
	locked := filepath.Join(directory, "package-lock.json")
	secret := `"token": "ghp_16C7e42F292c6912E7710c838347Ae178B4a"`
	if err := os.WriteFile(locked, []byte(secret), 0o600); err != nil {
		t.Fatal(err)
	}

	findings, err := config.ScanPath(directory)
	if err != nil {
		t.Fatalf("ScanPath: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("lockfile should be allowlisted, got %+v", findings)
	}
}

func TestScanPathFindsSecretInFile(t *testing.T) {
	config := loadConfig(t)

	directory := t.TempDir()
	source := filepath.Join(directory, "settings.py")
	if err := os.WriteFile(source,
		[]byte(`GITHUB_TOKEN = "ghp_16C7e42F292c6912E7710c838347Ae178B4a"`), 0o600); err != nil {
		t.Fatal(err)
	}

	findings, err := config.ScanPath(directory)
	if err != nil {
		t.Fatalf("ScanPath: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	if findings[0].RuleID != "github-pat" || findings[0].Line != 1 {
		t.Fatalf("unexpected finding: %+v", findings[0])
	}
}

func TestRedactedNeverPrintsWholeSecret(t *testing.T) {
	finding := Finding{Secret: "ghp_16C7e42F292c6912E7710c838347Ae178B4a"}
	redacted := finding.Redacted()

	if strings.Contains(redacted, "16C7e42F") {
		t.Fatalf("redaction leaks the secret: %s", redacted)
	}
	if !strings.HasPrefix(redacted, "ghp_") {
		t.Fatalf("redaction should keep a locating prefix: %s", redacted)
	}
}

func TestBinaryFilesSkipped(t *testing.T) {
	config := loadConfig(t)

	directory := t.TempDir()
	binary := filepath.Join(directory, "blob.dat")
	if err := os.WriteFile(binary,
		append([]byte("ghp_16C7e42F292c6912E7710c838347Ae178B4a"), 0x00, 0x01), 0o600); err != nil {
		t.Fatal(err)
	}

	findings, err := config.ScanPath(directory)
	if err != nil {
		t.Fatalf("ScanPath: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("binary file should be skipped, got %+v", findings)
	}
}
