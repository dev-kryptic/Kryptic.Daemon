//go:build linux

package dialog

import (
	"os/exec"
	"strings"
)

func Info(title, message string) {
	if runZenity("info", title, message, "") {
		return
	}
	if _, err := exec.LookPath("kdialog"); err == nil {
		_ = exec.Command("kdialog", "--title", title, "--msgbox", message).Run()
	}
}

func Confirm(title, message string) bool {
	if bin := firstOf("zenity", "qarma", "yad"); bin != "" {
		cmd := exec.Command(bin, "--question", "--title="+title, "--text="+message)
		return cmd.Run() == nil
	}
	if _, err := exec.LookPath("kdialog"); err == nil {
		return exec.Command("kdialog", "--title", title, "--yesno", message).Run() == nil
	}
	return false
}

func Prompt(title, message, defaultValue string) (string, bool) {
	if bin := firstOf("zenity", "qarma", "yad"); bin != "" {
		cmd := exec.Command(bin, "--entry", "--title="+title, "--text="+message, "--entry-text="+defaultValue)
		out, err := cmd.Output()
		if err != nil {
			return "", false
		}
		return strings.TrimSpace(string(out)), true
	}
	if _, err := exec.LookPath("kdialog"); err == nil {
		cmd := exec.Command("kdialog", "--title", title, "--inputbox", message, defaultValue)
		out, err := cmd.Output()
		if err != nil {
			return "", false
		}
		return strings.TrimSpace(string(out)), true
	}
	return "", false
}

func runZenity(kind, title, message, extra string) bool {
	bin := firstOf("zenity", "qarma", "yad")
	if bin == "" {
		return false
	}
	args := []string{"--" + kind, "--title=" + title, "--text=" + message, "--width=380"}
	if extra != "" {
		args = append(args, extra)
	}
	return exec.Command(bin, args...).Run() == nil
}

func firstOf(names ...string) string {
	for _, name := range names {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}
