//go:build !darwin && !windows

package applog

import (
	"os/exec"
	"path/filepath"
)

func revealPath(path string) error {
	return exec.Command("xdg-open", filepath.Dir(path)).Start()
}
