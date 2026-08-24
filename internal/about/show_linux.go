//go:build linux

package about

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var showing sync.Mutex

func show() {
	if !showing.TryLock() {
		return
	}
	defer showing.Unlock()

	icon := writeLogoTemp()
	if icon != "" {
		defer os.Remove(icon)
	}

	markup := fmt.Sprintf(
		"<span size='x-large' weight='bold'>%s</span>\n"+
			"<span>%s</span>\n\n"+
			"<span size='small' foreground='#6b7280'>%s</span>\n\n"+
			"%s\n\n"+
			"<a href='%s'>%s</a>",
		escapePango(AppName),
		escapePango(Tagline),
		escapePango(VersionLine()),
		escapePango(Blurb),
		WebsiteURL,
		escapePango(WebsiteLabel),
	)

	if runZenityFamily(markup, icon) {
		return
	}
	if runKDialog() {
		return
	}
	OpenWebsite()
}

func runZenityFamily(markup, icon string) bool {
	for _, bin := range []string{"zenity", "qarma", "yad"} {
		if _, err := exec.LookPath(bin); err != nil {
			continue
		}
		args := []string{
			"--info",
			"--title=" + WindowTitle,
			"--width=380",
			"--text=" + markup,
			"--ok-label=Close",
			"--extra-button=" + WebsiteLabel,
		}
		if icon != "" {
			switch bin {
			case "yad":
				args = append(args, "--window-icon="+icon)
			default:
				args = append(args, "--window-icon="+icon)
			}
		}
		cmd := exec.Command(bin, args...)
		out, _ := cmd.CombinedOutput()
		if strings.Contains(string(out), WebsiteLabel) {
			OpenWebsite()
		}
		return true
	}
	return false
}

func runKDialog() bool {
	if _, err := exec.LookPath("kdialog"); err != nil {
		return false
	}
	text := fmt.Sprintf("%s\n%s\n\n%s\n\n%s\n\n%s", AppName, Tagline, VersionLine(), Blurb, WebsiteURL)
	cmd := exec.Command("kdialog", "--title", WindowTitle, "--yesnocancel", text, "--yes-label", WebsiteLabel, "--no-label", "Close", "--cancel-label", "Close")
	err := cmd.Run()
	if err == nil {
		OpenWebsite()
	}
	return true
}

func writeLogoTemp() string {
	dir := os.TempDir()
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		dir = runtimeDir
	}
	path := filepath.Join(dir, "kryptic-about-logo.png")
	if err := os.WriteFile(path, logoPNG, 0600); err != nil {
		return ""
	}
	return path
}

func escapePango(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}
