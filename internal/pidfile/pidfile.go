// Package pidfile tracks the running daemon's process id so `kryptic stop`
// can find it. The file lives next to the session file under the user config
// directory and is best-effort: a crashed daemon leaves a stale file, which
// readers detect by probing the process.
package pidfile

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var ErrNotRunning = errors.New("daemon is not running (no pidfile)")

func path() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "kryptic", "daemon.pid"), nil
}

// Write records the current process id. Called by `kryptic start`.
func Write() error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(strconv.Itoa(os.Getpid())), 0o600)
}

// Remove deletes the pidfile, but only if it still belongs to this process -
// a replacement daemon may have already overwritten it.
func Remove() {
	p, err := path()
	if err != nil {
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return
	}
	if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid == os.Getpid() {
		_ = os.Remove(p)
	}
}

// Read returns the recorded pid, or ErrNotRunning when there is none.
func Read() (int, error) {
	p, err := path()
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return 0, ErrNotRunning
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, ErrNotRunning
	}
	return pid, nil
}

// Clear removes the pidfile regardless of owner - used by `kryptic stop`
// after the daemon exits, and to clean up stale files.
func Clear() {
	if p, err := path(); err == nil {
		_ = os.Remove(p)
	}
}
