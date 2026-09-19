//go:build windows

package termopen

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func openTerminal(bin string, args []string) error {
	line := quoteWindows(bin)
	for _, arg := range args {
		line += " " + quoteWindows(arg)
	}
	if wt, err := exec.LookPath("wt.exe"); err == nil {
		cmd := exec.Command(wt, "--title", "Dynamic-Secrets Connector", "--", "cmd", "/k", line)
		hideConsole(cmd)
		if err := cmd.Start(); err == nil {
			return nil
		}
	}
	cmd := exec.Command("cmd", "/c", "start", "Dynamic-Secrets Connector", "cmd", "/k", line)
	hideConsole(cmd)
	return cmd.Start()
}

func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
}
