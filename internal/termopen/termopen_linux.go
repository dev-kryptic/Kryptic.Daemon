//go:build linux

package termopen

import (
	"fmt"
	"os/exec"
)

func openTerminal(bin string, args []string) error {
	argv := append([]string{bin}, args...)
	line := shellJoin(argv)

	argvLaunchers := []struct {
		name string
		head []string
	}{
		{"gnome-terminal", []string{"--"}},
		{"kgx", []string{"--"}},
		{"ptyxis", []string{"--"}},
		{"mate-terminal", []string{"--"}},
		{"konsole", []string{"-e"}},
		{"terminator", []string{"-x"}},
		{"xterm", []string{"-e"}},
	}
	for _, launcher := range argvLaunchers {
		if startNamed(launcher.name, append(launcher.head, argv...)...) {
			return nil
		}
	}

	lineLaunchers := [][]string{
		{"xfce4-terminal", "-e", line},
		{"lxterminal", "-e", line},
		{"x-terminal-emulator", "-e", line},
	}
	for _, launcher := range lineLaunchers {
		if startNamed(launcher[0], launcher[1:]...) {
			return nil
		}
	}
	return fmt.Errorf("no terminal found (tried gnome-terminal, konsole, xterm)")
}

func startNamed(name string, args ...string) bool {
	if _, err := exec.LookPath(name); err != nil {
		return false
	}
	return exec.Command(name, args...).Start() == nil
}
