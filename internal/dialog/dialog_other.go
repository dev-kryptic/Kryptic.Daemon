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

func Prompt(title, message, defaultValue string) (string, bool) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", title, message)
	return "", false
}
