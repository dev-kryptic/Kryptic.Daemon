//go:build !windows

package pidfile

import (
	"os"
	"syscall"
)

// Alive reports whether the process exists (signal 0 probes without killing).
func Alive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

// Terminate asks the process to exit gracefully (SIGTERM).
func Terminate(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}
