//go:build linux

package main

import (
	"os"
	"path/filepath"
	"syscall"
)

func reexecIfLinux() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	_ = syscall.Exec(exe, os.Args, os.Environ())
}
