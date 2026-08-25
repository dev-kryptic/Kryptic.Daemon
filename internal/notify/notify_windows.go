//go:build windows

package notify

import (
	"os/exec"
	"strings"
)

func show(title, body string) {
	script := `Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$notify = New-Object System.Windows.Forms.NotifyIcon
$notify.Icon = [System.Drawing.SystemIcons]::Information
$notify.BalloonTipTitle = ` + psQuote(title) + `
$notify.BalloonTipText = ` + psQuote(body) + `
$notify.Visible = $true
$notify.ShowBalloonTip(10000)
Start-Sleep -Seconds 10
$notify.Dispose()
`
	_ = exec.Command("powershell", "-NoProfile", "-STA", "-Command", script).Run()
}

func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
