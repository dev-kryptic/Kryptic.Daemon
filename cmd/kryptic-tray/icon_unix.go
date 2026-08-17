//go:build !windows

package main

import _ "embed"

//go:embed assets/kryptic.png
var trayIcon []byte
