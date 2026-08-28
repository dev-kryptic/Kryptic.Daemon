//go:build !windows

// Package singleinstance stops a second copy of a desktop process from
// running for the same OS user. The daemon socket cannot serve this purpose:
// a tray acting as a remote control for a systemd-run daemon holds no socket,
// so launching the app twice used to register two tray icons.
package singleinstance

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/sys/unix"
)

// Acquire takes the per-user lock for name. When ok, the returned release
// frees the lock; the kernel also releases it automatically when the process
// exits or execs (the fd is close-on-exec). When not ok, another instance
// holds the lock.
func Acquire(name string) (release func(), ok bool) {
	file, err := os.OpenFile(lockPath(name), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		// No lock beats refusing to start: run without the guard.
		return func() {}, true
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, false
	}
	return func() { _ = file.Close() }, true
}

func lockPath(name string) string {
	if runtime.GOOS == "linux" {
		if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
			return filepath.Join(runtimeDir, name+".lock")
		}
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d.lock", name, os.Getuid()))
}
