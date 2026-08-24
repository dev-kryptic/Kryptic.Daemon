//go:build windows

package main

import (
	_ "embed"
	"time"

	"fyne.io/systray"
	"golang.org/x/sys/windows/registry"
)

// Windows tray icons must be ICO format. The taskbar never recolors them, so
// a dark taskbar needs the white falcon and a light taskbar the black one.
//
//go:embed assets/kryptic.ico
var trayIconBlack []byte

//go:embed assets/kryptic-white.ico
var trayIconWhite []byte

func currentTrayIcon() []byte {
	if taskbarIsLight() {
		return trayIconBlack
	}
	return trayIconWhite
}

// taskbarIsLight reads the personalization setting the taskbar follows
// (SystemUsesLightTheme, not AppsUseLightTheme). A missing value means the
// legacy always-dark taskbar.
func taskbarIsLight() bool {
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return false
	}
	defer key.Close()
	value, _, err := key.GetIntegerValue("SystemUsesLightTheme")
	return err == nil && value == 1
}

// watchTrayTheme swaps the icon when the user flips Windows dark/light mode.
func watchTrayTheme() {
	light := taskbarIsLight()
	for range time.Tick(5 * time.Second) {
		now := taskbarIsLight()
		if now != light {
			light = now
			systray.SetIcon(currentTrayIcon())
		}
	}
}
