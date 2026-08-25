//go:build darwin

package notify

import (
	"os/exec"
	"strings"
)

func show(title, body string) {
	script := "display notification " + quoteApple(body) + " with title " + quoteApple(title)
	_ = exec.Command("osascript", "-e", script).Start()
}

func quoteApple(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
