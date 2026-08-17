//go:build darwin

package authstore

import (
	"errors"
	"os/exec"
	"strings"
)

const (
	keychainService = "dev.kryptic.daemon"
	keychainAccount = "refresh-token"
)

func platformSave(refreshToken string) error {
	// -U updates an existing item instead of failing on duplicates.
	cmd := exec.Command("/usr/bin/security", "add-generic-password",
		"-U", "-s", keychainService, "-a", keychainAccount, "-w", refreshToken)
	if output, err := cmd.CombinedOutput(); err != nil {
		return errors.New("keychain write failed: " + strings.TrimSpace(string(output)))
	}
	return nil
}

func platformLoad() (string, error) {
	output, err := exec.Command("/usr/bin/security", "find-generic-password",
		"-s", keychainService, "-a", keychainAccount, "-w").Output()
	if err != nil {
		return "", ErrNotLoggedIn
	}
	return strings.TrimSpace(string(output)), nil
}

func platformClear() {
	_ = exec.Command("/usr/bin/security", "delete-generic-password",
		"-s", keychainService, "-a", keychainAccount).Run()
}
