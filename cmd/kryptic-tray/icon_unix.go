//go:build !windows

package main

import _ "embed"

//go:embed assets/kryptic.png
var trayIcon []byte

func currentTrayIcon() []byte { return trayIcon }

// watchTrayTheme is Windows-only. Freedesktop status-notifier trays recolor
// or badge icons themselves, so there is nothing to watch here.
func watchTrayTheme() {}
