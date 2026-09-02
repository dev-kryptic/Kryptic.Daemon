//go:build windows

package dialog

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var procShellExecuteW = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteW")

// OpenPath opens path in the OS default application.
func OpenPath(path string) {
	if path == "" {
		return
	}
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return
	}
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	procShellExecuteW.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)), 0, 0, 1)
}
