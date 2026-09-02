package authstore

import (
	"path/filepath"
)

func lockPath() (string, error) {
	path, err := filePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "session.lock"), nil
}
