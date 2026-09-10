//go:build darwin

package authstore

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const (
	keychainService = "dev.kryptic.daemon"
	keychainAccount = "refresh-token"
)

func platformSave(refreshToken string) error {
	// `security -i` reads the command from stdin, keeping the session out of
	// argv where any local process could read it via ps. -U updates an
	// existing item instead of failing on duplicates. The interactive parser
	// honors double quotes with backslash escapes and exits with the status
	// of the failed command, so errors surface the same as one-shot mode.
	command := fmt.Sprintf("add-generic-password -U -s %s -a %s -w %s\n",
		keychainService, keychainAccount, securityQuote(refreshToken))
	cmd := exec.Command("/usr/bin/security", "-i")
	cmd.Stdin = strings.NewReader(command)
	if output, err := cmd.CombinedOutput(); err != nil {
		return errors.New("keychain write failed: " + strings.TrimSpace(string(output)))
	}
	return nil
}

// securityQuote wraps a value for the `security -i` command parser:
// backslash escapes inside double quotes.
func securityQuote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
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
