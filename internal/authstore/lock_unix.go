//go:build unix

package authstore

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// WithLock serializes refresh-token read/rotate/save across the daemon and CLI
// so two processes cannot spend the same rotating token.
func WithLock(fn func() error) error {
	path, err := lockPath()
	if err != nil {
		return fn()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fn()
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fn()
	}
	defer file.Close()

	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		return fn()
	}
	defer unix.Flock(int(file.Fd()), unix.LOCK_UN)
	return fn()
}
