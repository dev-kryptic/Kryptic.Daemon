//go:build windows

package update

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// openWindowsInstaller starts the setup exe outside the tray's process tree.
// CREATE_BREAKAWAY_FROM_JOB matters: if the installer stayed a child of
// kryptic-tray.exe, Inno's taskkill /T of the tray would kill the installer too.
func openWindowsInstaller(path string) error {
	cmd := exec.Command(path)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_BREAKAWAY_FROM_JOB,
	}
	return cmd.Start()
}
