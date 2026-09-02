package scan

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const progressBarWidth = 24

// TerminalProgress writes a 0-100% bar to stderr. When stderr is not a
// terminal (CI, pipes, git hooks) it returns nil so the scan stays quiet.
func TerminalProgress() Progress {
	info, err := os.Stderr.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return nil
	}

	color := os.Getenv("NO_COLOR") == ""
	return func(percent int, message string) {
		if percent < 0 {
			percent = 0
		}
		if percent > 100 {
			percent = 100
		}
		filledCount := percent * progressBarWidth / 100
		filled := strings.Repeat("█", filledCount)
		empty := strings.Repeat("░", progressBarWidth-filledCount)
		bar := filled + empty
		if color {
			bar = "\033[32m" + filled + "\033[0m" + empty
		}
		fmt.Fprintf(os.Stderr, "\r  %s  %3d%%  %s\033[K", bar, percent, truncateStatus(message, 48))
		if percent >= 100 {
			fmt.Fprintln(os.Stderr)
		}
	}
}

// LineProgress writes one "percent<TAB>message" line per update. The menu-bar
// app uses `--progress` so it can drive a determinate bar when stderr is a
// pipe (not a TTY). Hooks and CI omit the flag and stay quiet.
func LineProgress(w io.Writer) Progress {
	if w == nil {
		return nil
	}
	return func(percent int, message string) {
		if percent < 0 {
			percent = 0
		}
		if percent > 100 {
			percent = 100
		}
		fmt.Fprintf(w, "%d\t%s\n", percent, strings.ReplaceAll(message, "\n", " "))
	}
}

func truncateStatus(value string, max int) string {
	if max < 4 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return "…" + string(runes[len(runes)-(max-1):])
}
