//go:build linux

package authstore

import (
	"errors"
	"os/exec"
	"strings"
)

// libsecret via secret-tool (package libsecret-tools on Debian/Ubuntu,
// libsecret on Arch/Fedora). Exec'ing the system tool keeps the daemon
// cgo-free and lets libsecret handle keyring unlock prompts; a missing
// binary or Secret Service simply falls back to the 0600 file.
const (
	secretToolService = "dev.kryptic.daemon"
	secretToolAccount = "refresh-token"
)

func platformSave(refreshToken string) error {
	tool, err := exec.LookPath("secret-tool")
	if err != nil {
		return errors.New("secret-tool not installed")
	}

	cmd := exec.Command(tool, "store", "--label=Kryptic daemon refresh token",
		"service", secretToolService, "account", secretToolAccount)
	cmd.Stdin = strings.NewReader(refreshToken)
	if output, err := cmd.CombinedOutput(); err != nil {
		return errors.New("secret-tool store failed: " + strings.TrimSpace(string(output)))
	}
	return nil
}

func platformLoad() (string, error) {
	tool, err := exec.LookPath("secret-tool")
	if err != nil {
		return "", ErrNotLoggedIn
	}

	output, err := exec.Command(tool, "lookup",
		"service", secretToolService, "account", secretToolAccount).Output()
	if err != nil {
		return "", ErrNotLoggedIn
	}
	token := strings.TrimSpace(string(output))
	if token == "" {
		return "", ErrNotLoggedIn
	}
	return token, nil
}

func platformClear() {
	if tool, err := exec.LookPath("secret-tool"); err == nil {
		_ = exec.Command(tool, "clear",
			"service", secretToolService, "account", secretToolAccount).Run()
	}
}
