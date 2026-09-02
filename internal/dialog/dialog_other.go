//go:build !linux && !windows

package dialog

import (
	"fmt"
	"os"
)

func OpenProgress(string, string) Progress { return nopProgress{} }

func Info(title, message string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", title, message)
}

func Confirm(title, message string) bool {
	fmt.Fprintf(os.Stderr, "%s: %s\n", title, message)
	return false
}

func Ask(title, message, accept, reject string) bool {
	fmt.Fprintf(os.Stderr, "%s: %s (%s / %s)\n", title, message, accept, reject)
	return false
}

func Prompt(title, message, defaultValue string) (string, bool) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", title, message)
	return "", false
}

func PickFolder(title string) (string, bool) {
	fmt.Fprintf(os.Stderr, "%s: folder picker is not available on this platform\n", title)
	return "", false
}

func OpenPath(path string) {
	fmt.Fprintf(os.Stderr, "open %s\n", path)
}
