//go:build darwin

package applog

import "os/exec"

func revealPath(path string) error {
	return exec.Command("open", "-R", path).Start()
}
