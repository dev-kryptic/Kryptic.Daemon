//go:build linux

package about

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
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

	// yad first: it is the only dialog tool that can render the macOS-style
	// layout (logo above centered text with custom buttons).
	if runYad(markup, icon) {
		return
	}
	if runZenityFamily(markup, icon) {
		return
	}
	if runKDialog() {
		return
	}
	OpenWebsite()
}

func runYad(markup, icon string) bool {
	if _, err := exec.LookPath("yad"); err != nil {
		return false
	}
	args := []string{
		"--title=" + WindowTitle,
		"--text=" + markup,
		"--text-align=center",
		"--button=" + WebsiteLabel + ":2",
		"--button=Close:0",
		"--buttons-layout=center",
		"--width=380",
		"--borders=24",
		"--center",
		"--fixed",
	}
	if icon != "" {
		args = append(args, "--window-icon="+icon, "--image="+icon, "--image-on-top")
	}
	err := exec.Command("yad", args...).Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
		OpenWebsite()
	}
	return true
}

func runZenityFamily(markup, icon string) bool {
	for _, bin := range []string{"zenity", "qarma"} {
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
			args = append(args, "--window-icon="+icon)
		}
		out, _ := exec.Command(bin, args...).CombinedOutput()
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

// writeLogoTemp writes a 128px copy of the logo: yad renders images at native
// size, and the embedded asset is 1000x1000.
func writeLogoTemp() string {
	scaled, err := scaledLogoPNG(128)
	if err != nil {
		return ""
	}
	dir := os.TempDir()
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		dir = runtimeDir
	}
	path := filepath.Join(dir, "kryptic-about-logo.png")
	if err := os.WriteFile(path, scaled, 0600); err != nil {
		return ""
	}
	return path
}

func scaledLogoPNG(size int) ([]byte, error) {
	src, err := png.Decode(bytes.NewReader(logoPNG))
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			out.Set(x, y, src.At(b.Min.X+x*b.Dx()/size, b.Min.Y+y*b.Dy()/size))
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func escapePango(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}
