//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

const autostartValueName = "Kryptic"

// ensureAutostart registers the tray under HKCU\...\Run so it starts at
// login, the Windows counterpart of ~/.config/autostart on Linux and the
// login item on macOS. The installer cannot own this alone: in-place updates
// never re-run it, and an elevated installer writes the elevating user's
// hive, which is the wrong one when a standard user installs with an
// admin's credentials. Windows keeps the Task Manager startup toggle in a
// separate StartupApproved key, so re-writing this value never overrides a
// user who turned autostart off.
func ensureAutostart() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return
	}
	defer key.Close()

	value := `"` + exe + `"`
	if existing, _, err := key.GetStringValue(autostartValueName); err == nil && existing == value {
		return
	}
	_ = key.SetStringValue(autostartValueName, value)
}
