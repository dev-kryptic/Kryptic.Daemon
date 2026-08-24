// Package config is the daemon's persisted settings: today the Daemon BFF URL.
// KRYPTIC_API always wins over the file, which wins over the hosted default.
package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultAPI = "https://daemon.kryptic.dev"

	SourceEnvironment = "environment"
	SourceFile        = "config"
	SourceDefault     = "default"
)

// File is the on-disk document. Unknown keys are ignored on read so we can
// add fields later without breaking older daemons.
type File struct {
	API string `json:"api,omitempty"`
}

// Dir is the per-user config directory (…/kryptic). KRYPTIC_CONFIG_DIR
// overrides it, which is how tests isolate the file.
func Dir() (string, error) {
	if override := strings.TrimSpace(os.Getenv("KRYPTIC_CONFIG_DIR")); override != "" {
		return override, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "kryptic"), nil
}

func path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() (File, error) {
	p, err := path()
	if err != nil {
		return File{}, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return File{}, nil
		}
		return File{}, err
	}
	var file File
	if err := json.Unmarshal(data, &file); err != nil {
		return File{}, fmt.Errorf("config file %s: %w", p, err)
	}
	return file, nil
}

func save(file File) error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(p, data, 0o600)
}

// NormalizeAPI accepts a Daemon BFF URL and returns a canonical form
// (scheme + host, no trailing slash).
func NormalizeAPI(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("server URL is empty")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid server URL %q", raw)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("server URL must be http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("invalid server URL %q", raw)
	}
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

// API returns the Daemon BFF the process should talk to, and where that
// value came from.
func API() (string, string) {
	if override := strings.TrimSpace(os.Getenv("KRYPTIC_API")); override != "" {
		if normalized, err := NormalizeAPI(override); err == nil {
			return normalized, SourceEnvironment
		}
		return strings.TrimRight(override, "/"), SourceEnvironment
	}
	file, err := Load()
	if err == nil {
		if normalized, err := NormalizeAPI(file.API); err == nil {
			return normalized, SourceFile
		}
	}
	return DefaultAPI, SourceDefault
}

// SetAPI writes the Daemon BFF URL. It does not touch the login session;
// callers that change the URL should sign the user out of the previous server.
func SetAPI(raw string) error {
	normalized, err := NormalizeAPI(raw)
	if err != nil {
		return err
	}
	file, err := Load()
	if err != nil {
		return err
	}
	file.API = normalized
	return save(file)
}

// ResetAPI removes a saved URL so the hosted default (or KRYPTIC_API) applies.
func ResetAPI() error {
	file, err := Load()
	if err != nil {
		return err
	}
	if file.API == "" {
		return nil
	}
	file.API = ""
	return save(file)
}

// EnvOverrides reports whether KRYPTIC_API is set and therefore ignores the file.
func EnvOverrides() bool {
	return strings.TrimSpace(os.Getenv("KRYPTIC_API")) != ""
}
