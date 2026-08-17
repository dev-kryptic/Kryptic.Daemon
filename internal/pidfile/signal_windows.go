//go:build windows

package pidfile

import (
	"os"

	"golang.org/x/sys/windows"
)

// Alive reports whether the process exists.
func Alive(pid int) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	var code uint32
	if err := windows.GetExitCodeProcess(handle, &code); err != nil {
		return false
	}
	return code == 259 // STILL_ACTIVE
}

// Terminate kills the process - Windows has no SIGTERM equivalent for
// console-less services, and the daemon holds no state that needs a
// graceful shutdown (secrets live in memory only).
func Terminate(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Kill()
}
