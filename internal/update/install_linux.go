//go:build linux

package update

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func privilegedInstall(files []stagedFile) error {
	if len(files) == 0 {
		return nil
	}
	script := installScript(files)
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
			return fmt.Errorf("could not replace files in %s: %w", files[0].dest, err)
		}
	}
	return fmt.Errorf("cannot write %s (permission denied). Allow the password prompt, or re-run from a terminal as root", files[0].dest)
}

func graphical() bool {
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

func installScript(files []stagedFile) string {
	parts := make([]string, 0, len(files))
	for _, file := range files {
		src := shellQuote(file.staging)
		dest := shellQuote(file.dest)
		// The elevated step re-verifies the checksum computed after download:
		// even if the staging file were somehow swapped between verification
		// and elevation, root refuses to install a payload that no longer
		// matches.
		check := shellQuote(file.sha256 + "  " + file.staging)
		parts = append(parts, fmt.Sprintf(
			"echo %s | sha256sum -c --status - && install -m 0755 %s %s.new && mv %s %s.old && mv %s.new %s && rm -f %s.old",
			check, src, dest, dest, dest, dest, dest, dest,
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
