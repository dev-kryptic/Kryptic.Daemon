//go:build linux

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dev-kryptic/daemon/internal/brand"
)

// ensureLauncherIcon writes the brand icon where the desktop entry's
// Icon=kryptic lookup can find it. Installers before 0.13.6 wrote the
// desktop entry without any icon file, so those launchers showed the
// generic gear. Running on every start also repairs installs that were
// updated in place, which never re-runs an installer.
func ensureLauncherIcon() {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return
	}
	data, err := brand.ScaledLogoPNG(256)
	if err != nil {
		return
	}

	targets := []string{
		filepath.Join(home, ".local/share/icons/hicolor/256x256/apps/kryptic.png"),
		filepath.Join(home, ".local/share/pixmaps/kryptic.png"),
	}
	changed := false
	for _, target := range targets {
		if existing, err := os.ReadFile(target); err == nil && bytes.Equal(existing, data) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			continue
		}
		if err := os.WriteFile(target, data, 0o644); err == nil {
			changed = true
		}
	}
	if !changed {
		return
	}
	// Refresh caches so the running shell picks the icon up without a
	// re-login. Both tools are optional; a stale cache fixes itself at the
	// next login.
	_ = exec.Command("gtk-update-icon-cache", "-q",
		filepath.Join(home, ".local/share/icons/hicolor")).Run()
	_ = exec.Command("update-desktop-database",
		filepath.Join(home, ".local/share/applications")).Run()
}
