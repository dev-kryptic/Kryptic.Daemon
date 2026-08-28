//go:build !windows

package main

// ensureAutostart is Windows-only. Linux autostarts via the XDG autostart
// desktop entry the installers write; macOS uses the app's launchd plist.
func ensureAutostart() {}
