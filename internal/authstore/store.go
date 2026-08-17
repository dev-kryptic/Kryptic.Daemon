// Package authstore keeps the daemon's session - the refresh token and the
// device's sealed-box key pair - in the OS credential store: the user Keychain
// on macOS (via /usr/bin/security), the Credential Manager on Windows
// (advapi32), and libsecret on Linux (via secret-tool). When the platform
// store is unavailable (e.g. a headless Linux box without a Secret Service),
// it falls back to a 0600 file under the user config directory. Access tokens
// are never persisted - they live in daemon memory only.
package authstore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var ErrNotLoggedIn = errors.New("not logged in - run `kryptic login` first")

// Session is everything the daemon persists between runs. The device private
// key opens the org-key grant sealed to this device (end-to-end encryption);
// it exists nowhere else, so clearing the session also revokes the device's
// ability to decrypt.
type Session struct {
	RefreshToken string `json:"refreshToken"`
	// Base64url 65-byte uncompressed SEC1 P-256 point, as registered at login.
	DevicePublicKey string `json:"devicePublicKey,omitempty"`
	// Base64url 32-byte P-256 scalar. Never leaves this machine.
	DevicePrivateKey string `json:"devicePrivateKey,omitempty"`
}

// SaveSession stores the session in the platform credential store, falling
// back to the config-dir file when the store is unavailable.
func SaveSession(session Session) error {
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	if err := platformSave(string(data)); err == nil {
		// A stale file copy must not shadow the platform store on later loads.
		if path, pathErr := filePath(); pathErr == nil {
			_ = os.Remove(path)
		}
		return nil
	}
	return saveFile(string(data))
}

// LoadSession reads the session, preferring the platform credential store.
func LoadSession() (Session, error) {
	raw, err := platformLoad()
	if err != nil || raw == "" {
		raw, err = loadFile()
		if err != nil {
			return Session{}, err
		}
	}
	return decodeSession(raw)
}

func decodeSession(raw string) (Session, error) {
	var session Session
	if json.Unmarshal([]byte(raw), &session) != nil || session.RefreshToken == "" {
		return Session{}, ErrNotLoggedIn
	}
	return session, nil
}

// Clear removes the session from both the platform store and the file
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
