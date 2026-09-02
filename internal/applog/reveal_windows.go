//go:build windows

package applog

import "os/exec"

func revealPath(path string) error {
	return exec.Command("explorer", "/select,", path).Start()
}
