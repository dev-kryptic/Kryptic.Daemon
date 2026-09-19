//go:build darwin

package termopen

import (
	"fmt"
	"os/exec"
)

func openTerminal(bin string, args []string) error {
	command := shellJoin(append([]string{bin}, args...))
	script := fmt.Sprintf(
		`set p to %q
tell application "Terminal"
	activate
	do script ("exec " & p)
end tell`, command)
	return exec.Command("osascript", "-e", script).Start()
}
