//go:build windows

package main

import _ "embed"

// Windows tray icons must be ICO format.
//
//go:embed assets/kryptic.ico
var trayIcon []byte
