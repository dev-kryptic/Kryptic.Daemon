//go:build windows

package dialog

import (
	"os"
	"os/exec"
	"strings"
)

func Info(title, message string) {
	_ = powershell(`
Add-Type -AssemblyName System.Windows.Forms
[void][System.Windows.Forms.MessageBox]::Show($env:KRYPTIC_MESSAGE, $env:KRYPTIC_TITLE)
`, title, message, "")
}

func Confirm(title, message string) bool {
	return powershell(`
Add-Type -AssemblyName System.Windows.Forms
$r = [System.Windows.Forms.MessageBox]::Show($env:KRYPTIC_MESSAGE, $env:KRYPTIC_TITLE, 'YesNo', 'Question')
if ($r -eq 'Yes') { exit 0 } else { exit 1 }
`, title, message, "") == nil
}

func Prompt(title, message, defaultValue string) (string, bool) {
	out, err := powershellOutput(`
Add-Type -AssemblyName Microsoft.VisualBasic
Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.Application]::EnableVisualStyles()
$r = [Microsoft.VisualBasic.Interaction]::InputBox($env:KRYPTIC_MESSAGE, $env:KRYPTIC_TITLE, $env:KRYPTIC_DEFAULT)
[Console]::Out.Write($r)
`, title, message, defaultValue)
	if err != nil {
		return "", false
	}
	if out == "" {
		return "", false
	}
	return out, true
}

func powershell(script, title, message, defaultValue string) error {
	cmd := exec.Command("powershell", "-NoProfile", "-STA", "-Command", script)
	cmd.Env = append(os.Environ(),
		"KRYPTIC_TITLE="+title,
		"KRYPTIC_MESSAGE="+message,
		"KRYPTIC_DEFAULT="+defaultValue,
	)
	return cmd.Run()
}

func powershellOutput(script, title, message, defaultValue string) (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-STA", "-Command", script)
	cmd.Env = append(os.Environ(),
		"KRYPTIC_TITLE="+title,
		"KRYPTIC_MESSAGE="+message,
		"KRYPTIC_DEFAULT="+defaultValue,
	)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
