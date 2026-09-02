//go:build linux

package dialog

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
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
	z := &zenityProgress{
		cmd:      cmd,
		w:        stdin,
		canceled: make(chan struct{}),
		done:     make(chan struct{}),
	}
	go func() {
		err := cmd.Wait()
		exitCode := 0
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				exitCode = ee.ExitCode()
			} else {
				exitCode = -1
			}
		}
		z.mu.Lock()
		closedByUs := z.closed
		z.mu.Unlock()
		// zenity/qarma/yad return 1 when the user hits Cancel or closes the window.
		if !closedByUs && exitCode == 1 {
			z.cancelOnce.Do(func() { close(z.canceled) })
		}
		close(z.done)
	}()
	return z
}

type zenityProgress struct {
	cmd        *exec.Cmd
	w          io.WriteCloser
	canceled   chan struct{}
	done       chan struct{}
	mu         sync.Mutex
	closed     bool
	cancelOnce sync.Once
	closeOnce  sync.Once
}

func (z *zenityProgress) Set(percent int, message string) {
	if z == nil {
		return
	}
	z.mu.Lock()
	w := z.w
	z.mu.Unlock()
	if w == nil {
		return
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	_, _ = fmt.Fprintf(w, "# %s\n%d\n", message, percent)
}

func (z *zenityProgress) Close() {
	if z == nil {
		return
	}
	z.closeOnce.Do(func() {
		z.mu.Lock()
		z.closed = true
		w := z.w
		z.w = nil
		z.mu.Unlock()
		if w != nil {
			_, _ = fmt.Fprint(w, "100\n")
			_ = w.Close()
		}
		<-z.done
	})
}

func (z *zenityProgress) Canceled() <-chan struct{} {
	if z == nil {
		return nil
	}
	return z.canceled
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
	return Ask(title, message, "", "")
}

func Ask(title, message, accept, reject string) bool {
	if bin := firstOf("zenity", "qarma", "yad"); bin != "" {
		args := []string{"--question", "--title=" + title, "--text=" + message, "--width=420"}
		if accept != "" {
			args = append(args, "--ok-label="+accept)
		}
		if reject != "" {
			args = append(args, "--cancel-label="+reject)
		}
		return exec.Command(bin, args...).Run() == nil
	}
	if _, err := exec.LookPath("kdialog"); err == nil {
		args := []string{"--title", title, "--yesno", message}
		if accept != "" {
			args = append(args, "--yes-label", accept)
		}
		if reject != "" {
			args = append(args, "--no-label", reject)
		}
		return exec.Command("kdialog", args...).Run() == nil
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

func PickFolder(title string) (string, bool) {
	if bin := firstOf("zenity", "qarma", "yad"); bin != "" {
		cmd := exec.Command(bin, "--file-selection", "--directory", "--title="+title, "--width=520")
		out, err := cmd.Output()
		if err != nil {
			return "", false
		}
		path := strings.TrimSpace(string(out))
		return path, path != ""
	}
	if _, err := exec.LookPath("kdialog"); err == nil {
		start := "."
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			start = home
		}
		cmd := exec.Command("kdialog", "--title", title, "--getexistingdirectory", start)
		out, err := cmd.Output()
		if err != nil {
			return "", false
		}
		path := strings.TrimSpace(string(out))
		return path, path != ""
	}
	return "", false
}

func OpenPath(path string) {
	if path == "" {
		return
	}
	if _, err := exec.LookPath("xdg-open"); err == nil {
		_ = exec.Command("xdg-open", path).Start()
		return
	}
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
