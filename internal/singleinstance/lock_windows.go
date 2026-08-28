//go:build windows

package singleinstance

import (
	"errors"

	"golang.org/x/sys/windows"
)

// Acquire takes a per-session named mutex. When ok, the returned release
// frees it; Windows also releases it when the process exits. When not ok,
// another instance holds the mutex.
func Acquire(name string) (release func(), ok bool) {
	// "Local\" scopes the mutex to the current login session, matching the
	// per-user unix flock.
	nameW, err := windows.UTF16PtrFromString(`Local\` + name)
	if err != nil {
		return func() {}, true
	}
	handle, err := windows.CreateMutex(nil, false, nameW)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if handle != 0 {
			_ = windows.CloseHandle(handle)
		}
		return nil, false
	}
	if err != nil {
		// No lock beats refusing to start: run without the guard.
		return func() {}, true
	}
	return func() { _ = windows.CloseHandle(handle) }, true
}
