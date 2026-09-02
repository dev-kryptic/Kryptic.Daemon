//go:build windows

package authstore

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
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

	var overlapped windows.Overlapped
	err = windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &overlapped)
	if err != nil {
		return fn()
	}
	defer windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &overlapped)
	return fn()
}
