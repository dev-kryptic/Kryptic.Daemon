//go:build linux

package update

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func privilegedInstall(pairs [][2]string) error {
	if len(pairs) == 0 {
		return nil
	}
	script := installScript(pairs)
	if graphical() {
		if pkexec, err := exec.LookPath("pkexec"); err == nil {
			if err := runElevated(pkexec, script); err == nil {
				return nil
			}
		}
	}
	if sudo, err := exec.LookPath("sudo"); err == nil {
		if err := runElevated(sudo, script); err == nil {
			return nil
		} else {
			return fmt.Errorf("could not replace files in %s: %w", pairs[0][1], err)
		}
	}
	return fmt.Errorf("cannot write %s (permission denied). Allow the password prompt, or re-run from a terminal as root", pairs[0][1])
}

func graphical() bool {
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

func installScript(pairs [][2]string) string {
	parts := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		src := shellQuote(pair[0])
		dest := shellQuote(pair[1])
		parts = append(parts, fmt.Sprintf(
			"install -m 0755 %s %s.new && mv %s %s.old && mv %s.new %s && rm -f %s.old",
			src, dest, dest, dest, dest, dest, dest,
		))
	}
	return strings.Join(parts, " && ")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func runElevated(helper, script string) error {
	sh := "/bin/sh"
	if _, err := os.Stat(sh); err != nil {
		sh = "/usr/bin/sh"
	}
	cmd := exec.Command(helper, sh, "-c", script)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
