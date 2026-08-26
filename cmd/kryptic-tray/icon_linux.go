//go:build linux

package main

import (
	"os"
	"os/exec"
	"time"

	"fyne.io/systray"
	"github.com/godbus/dbus/v5"
)

func currentTrayIcon() []byte { return currentTrayPNG() }

// panelIsLight follows org.freedesktop.appearance color-scheme. StatusNotifier
// trays paint pixmaps as-is, so a dark GNOME/Ubuntu panel needs the white
// falcon. Unknown means dark: those panels are dark by default.
func panelIsLight() bool {
	if light, ok := portalPanelIsLight(); ok {
		return light
	}
	if light, ok := gsettingsPanelIsLight(); ok {
		return light
	}
	return gtkThemeIsLight(os.Getenv("GTK_THEME"))
}

func portalPanelIsLight() (bool, bool) {
	scheme, ok := portalColorScheme()
	if !ok {
		return false, false
	}
	return scheme == 2, true
}

func portalColorScheme() (uint32, bool) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return 0, false
	}
	obj := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	var value dbus.Variant
	err = obj.Call(
		"org.freedesktop.portal.Settings.ReadOne",
		0,
		"org.freedesktop.appearance",
		"color-scheme",
	).Store(&value)
	if err != nil {
		err = obj.Call(
			"org.freedesktop.portal.Settings.Read",
			0,
			"org.freedesktop.appearance",
			"color-scheme",
		).Store(&value)
	}
	if err != nil {
		return 0, false
	}
	return unwrapUint32(value)
}

func unwrapUint32(v dbus.Variant) (uint32, bool) {
	cur := any(v)
	for range 4 {
		switch n := cur.(type) {
		case uint32:
			return n, true
		case dbus.Variant:
			cur = n.Value()
		default:
			return 0, false
		}
	}
	return 0, false
}

func gsettingsPanelIsLight() (bool, bool) {
	out, err := exec.Command("gsettings", "get", "org.gnome.desktop.interface", "color-scheme").Output()
	if err != nil {
		return false, false
	}
	return parseGSettingsColorScheme(string(out))
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
