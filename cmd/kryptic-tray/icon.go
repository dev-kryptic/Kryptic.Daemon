package main

import (
	_ "embed"
	"log"
	"strings"
	"sync"

	"github.com/dev-kryptic/daemon/internal/trayicon"
)

const trayIconSize = 128

//go:embed assets/Falcon.svg
var falconWhiteSVG []byte

//go:embed assets/Falcon-black.svg
var falconBlackSVG []byte

var (
	iconMu   sync.Mutex
	pngWhite []byte
	pngBlack []byte
)

func currentTrayPNG() []byte {
	light := panelIsLight()
	iconMu.Lock()
	defer iconMu.Unlock()
	if light {
		if pngBlack == nil {
			pngBlack = rasterizeTray(falconBlackSVG)
		}
		return pngBlack
	}
	if pngWhite == nil {
		pngWhite = rasterizeTray(falconWhiteSVG)
	}
	return pngWhite
}

func rasterizeTray(svg []byte) []byte {
	png, err := trayicon.RasterPNG(svg, trayIconSize)
	if err != nil {
		log.Printf("tray icon: %v", err)
		return nil
	}
	return png
}

func parseGSettingsColorScheme(out string) (light bool, ok bool) {
	switch strings.Trim(strings.TrimSpace(out), `"'`) {
	case "prefer-light":
		return true, true
	case "prefer-dark", "default":
		return false, true
	default:
		return false, false
	}
}

func gtkThemeIsLight(theme string) bool {
	theme = strings.ToLower(theme)
	if theme == "" {
		return false
	}
	return !strings.Contains(theme, "dark")
}
