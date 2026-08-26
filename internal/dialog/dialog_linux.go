//go:build linux

package dialog

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
)

func OpenProgress(title, message string) Progress {
	bin := firstOf("zenity", "qarma", "yad")
	if bin == "" {
		return nopProgress{}
	}
	cmd := exec.Command(bin,
		"--progress",
		"--title="+title,
		"--text="+message,
		"--percentage=0",
		"--auto-close",
		"--no-cancel",
		"--width=380",
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nopProgress{}
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nopProgress{}
	}
	return &zenityProgress{cmd: cmd, w: stdin}
}

type zenityProgress struct {
	cmd *exec.Cmd
	w   io.WriteCloser
}

func (z *zenityProgress) Set(percent int, message string) {
	if z == nil || z.w == nil {
		return
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	_, _ = fmt.Fprintf(z.w, "# %s\n%d\n", message, percent)
}

func (z *zenityProgress) Close() {
	if z == nil {
		return
	}
	if z.w != nil {
		_, _ = fmt.Fprint(z.w, "100\n")
		_ = z.w.Close()
		z.w = nil
	}
	if z.cmd != nil {
		_ = z.cmd.Wait()
		z.cmd = nil
	}
}

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
