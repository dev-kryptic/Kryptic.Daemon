//go:build linux

package notify

import "os/exec"

func show(title, body string) {
	if _, err := exec.LookPath("notify-send"); err != nil {
		return
	}
	_ = exec.Command("notify-send", "--app-name=Kryptic", title, body).Start()
}
