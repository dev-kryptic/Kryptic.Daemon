//go:build linux

package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// reexecIfLinux replaces this process with the binary on disk, so an in-place
// update starts running immediately (new tray and, with it, the in-process
// daemon).
func reexecIfLinux() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	exe = execTarget(exe)
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	execErr := syscall.Exec(exe, os.Args, os.Environ())
	// Exec only returns on failure. Fall back to spawning the new binary and
	// exiting, which still gets the user onto the new version.
	log.Printf("re-exec of %s failed: %v", exe, execErr)
	cmd := exec.Command(exe, os.Args[1:]...)
	if cmd.Start() == nil {
		os.Exit(0)
	}
}
