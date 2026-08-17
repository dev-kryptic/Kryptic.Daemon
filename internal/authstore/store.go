// Package authstore keeps the daemon's refresh token in the OS credential
// store: the user Keychain on macOS (via /usr/bin/security), the Credential
// Manager on Windows (advapi32), and libsecret on Linux (via secret-tool).
// When the platform store is unavailable (e.g. a headless Linux box without a
// Secret Service), it falls back to a 0600 file under the user config
// directory. Access tokens are never persisted - they live in daemon memory
// only.
package authstore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var ErrNotLoggedIn = errors.New("not logged in - run `kryptic login` first")

// Save stores the refresh token in the platform credential store, falling
// back to the config-dir file when the store is unavailable.
func Save(refreshToken string) error {
	if err := platformSave(refreshToken); err == nil {
		// A stale file copy must not shadow the platform store on later loads.
		if path, pathErr := filePath(); pathErr == nil {
			_ = os.Remove(path)
		}
		return nil
	}
	return saveFile(refreshToken)
}

// Load reads the refresh token, preferring the platform credential store.
func Load() (string, error) {
	if token, err := platformLoad(); err == nil && token != "" {
		return token, nil
	}
	return loadFile()
}

// Clear removes the refresh token from both the platform store and the file
// fallback (whichever holds it).
func Clear() error {
	platformClear()
	if path, err := filePath(); err == nil {
		_ = os.Remove(path)
	}
	return nil
}

// ---------- file fallback ----------

func filePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "kryptic", "session"), nil
}

func saveFile(refreshToken string) error {
	path, err := filePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(refreshToken), 0o600)
}

func loadFile() (string, error) {
	path, err := filePath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ErrNotLoggedIn
	}
	return strings.TrimSpace(string(data)), nil
}
