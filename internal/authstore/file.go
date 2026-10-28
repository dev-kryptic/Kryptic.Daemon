package authstore

import (
	"os"
	"path/filepath"
	"strings"
)

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
		return "", ErrNotLoggedIn
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ErrNotLoggedIn
	}
	return strings.TrimSpace(string(data)), nil
}
