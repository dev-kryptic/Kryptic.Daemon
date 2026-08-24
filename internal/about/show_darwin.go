//go:build darwin

package about

import (
	"fmt"
	"os/exec"
	"strings"
)

func show() {
	body := fmt.Sprintf("%s\n%s\n\n%s\n\n%s", AppName, Tagline, VersionLine(), Blurb)
	script := fmt.Sprintf(
		`display dialog %s with title %s buttons {%s, "OK"} default button "OK"`,
		quoteApple(body),
		quoteApple(WindowTitle),
		quoteApple(WebsiteLabel),
	)
	out, err := exec.Command("osascript", "-e", script).Output()
	if err == nil && strings.Contains(string(out), WebsiteLabel) {
		OpenWebsite()
	}
}

func quoteApple(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
