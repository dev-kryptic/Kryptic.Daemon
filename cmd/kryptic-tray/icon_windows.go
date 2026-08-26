//go:build windows

package main

import (
	"time"

	"fyne.io/systray"
	"github.com/dev-kryptic/daemon/internal/trayicon"
	"golang.org/x/sys/windows/registry"
)

func currentTrayIcon() []byte {
	return trayicon.PNGToICO(currentTrayPNG(), trayIconSize)
}

// taskbarIsLight reads SystemUsesLightTheme (the taskbar setting, not
// AppsUseLightTheme). A missing value means the legacy always-dark taskbar.
func panelIsLight() bool {
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

func watchTrayTheme() {
	light := panelIsLight()
	for range time.Tick(5 * time.Second) {
		now := panelIsLight()
		if now != light {
			light = now
			systray.SetIcon(currentTrayIcon())
		}
	}
}
