//go:build linux

package manageui

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dev-kryptic/daemon/internal/brand"
)

//go:embed linuxpanel.py
var linuxPanelPy string

var showing sync.Mutex

// Show opens Open Kryptic. GTK (python3-gi) paints the same 380px panel as
// macOS and Windows. yad/zenity stay as a fallback on headless boxes.
func Show(h Handlers) {
	if !showing.TryLock() {
		return
	}
	beginSession()
	go func() {
		defer showing.Unlock()
		defer endSession()
		if runGTK(h) {
			return
		}
		for {
			snap := h.snap()
			action, arg, ok := pickAction(snap)
			if !ok || action == "" {
				return
			}
			h.fire(action, arg)
			if action == "quit" {
				return
			}
			time.Sleep(250 * time.Millisecond)
		}
	}()
}

func gtkReady() bool {
	if _, err := exec.LookPath("python3"); err != nil {
		return false
	}
	probes := []string{
		"import gi; gi.require_version('Gdk','3.0'); gi.require_version('Gtk','3.0'); from gi.repository import Gdk, Gtk, GLib",
		"import gi; gi.require_version('Gdk','4.0'); gi.require_version('Gtk','4.0'); from gi.repository import Gdk, Gtk, GLib",
	}
	for _, probe := range probes {
		if exec.Command("python3", "-c", probe).Run() == nil {
			return true
		}
	}
	return false
}

func runGTK(h Handlers) bool {
	if !gtkReady() {
		return false
	}
	dir, err := os.MkdirTemp("", "kryptic-panel-*")
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)
	script := filepath.Join(dir, "panel.py")
	logo := filepath.Join(dir, "logo.png")
	if err := os.WriteFile(script, []byte(linuxPanelPy), 0o644); err != nil {
		return false
	}
	if err := os.WriteFile(logo, brand.LogoPNG, 0o644); err != nil {
		return false
	}
	cmd := exec.Command("python3", script, logo)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return false
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return false
	}

	done := make(chan struct{})
	go func() {
		dec := json.NewDecoder(stdout)
		for {
			var action panelAction
			if err := dec.Decode(&action); err != nil {
				break
			}
			if !KnownAction(action.Name) {
				continue
			}
			h.fire(action.Name, action.Arg)
			if action.Name == "quit" {
				_ = cmd.Process.Kill()
				break
			}
		}
		close(done)
	}()

	enc := json.NewEncoder(stdin)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	writeSnap := func() {
		if err := enc.Encode(encodePanelSnap(h.snap())); err != nil {
			_ = cmd.Process.Kill()
		}
	}
	writeSnap()
	for {
		select {
		case <-done:
			_ = stdin.Close()
			_ = cmd.Wait()
			return true
		case <-tick.C:
			writeSnap()
		}
	}
}

func pickAction(snap Snapshot) (string, string, bool) {
	header := statusText(snap)
	if bin, err := exec.LookPath("yad"); err == nil {
		return runYad(bin, header, snap)
	}
	if bin := firstOf("zenity", "qarma"); bin != "" {
		return runZenityList(bin, header, snap)
	}
	return "", "", false
}

func statusText(snap Snapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  <b>%s</b>\n", statusDotMarkup(snap.Connection), snap.ConnectionLabel)
	if snap.Email != "" {
		fmt.Fprintf(&b, "%s\n", snap.Email)
	}
	if host := HostLabel(snap.API); host != "" {
		fmt.Fprintf(&b, "%s\n", host)
	}
	if snap.LoginCode != "" {
		fmt.Fprintf(&b, "Confirm code in browser: %s\n", snap.LoginCode)
	}
	if snap.LoginError != "" {
		fmt.Fprintf(&b, "%s\n", snap.LoginError)
	}
	return b.String()
}

func statusDotMarkup(kind string) string {
	switch kind {
	case "connected":
		return `<span foreground='#30d158'>●</span>`
	case "connecting", "awaiting_approval":
		return `<span foreground='#ff9f0a'>●</span>`
	default:
		return `<span foreground='#8e8e93'>○</span>`
	}
}

func runYad(bin, header string, snap Snapshot) (string, string, bool) {
	args := []string{
		"--title=Open Kryptic",
		"--width=380",
		"--height=560",
		"--center",
		"--text=" + header,
		"--list",
		"--no-headers",
		"--column=key:HD",
		"--column=Action",
		"--hide-column=1",
		"--print-column=1",
		"--button=Close:1",
	}
	rows := ActionRows(snap)
	for _, row := range rows {
		args = append(args, encodeRow(row), row.Label)
	}
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		return "", "", false
	}
	key := strings.TrimSpace(strings.TrimSuffix(string(out), "|"))
	if name, arg, ok := decodeRow(key); ok {
		return name, arg, true
	}
	for _, row := range rows {
		if row.Label == key {
			return row.Name, row.Arg, true
		}
	}
	return "", "", false
}

func runZenityList(bin, header string, snap Snapshot) (string, string, bool) {
	rows := ActionRows(snap)
	args := []string{
		"--list",
		"--title=Open Kryptic",
		"--text=" + stripMarkup(header),
		"--width=380",
		"--height=560",
		"--column=Action",
		"--hide-header",
	}
	for _, row := range rows {
		args = append(args, row.Label)
	}
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		return "", "", false
	}
	picked := strings.TrimSpace(string(out))
	for _, row := range rows {
		if row.Label == picked {
			return row.Name, row.Arg, true
		}
	}
	return "", "", false
}

func encodeRow(row Row) string {
	if row.Arg == "" {
		return row.Name
	}
	return row.Name + "\t" + row.Arg
}

func decodeRow(key string) (string, string, bool) {
	name, arg, _ := strings.Cut(key, "\t")
	if !KnownAction(name) {
		return "", "", false
	}
	return name, arg, true
}

func stripMarkup(s string) string {
	r := strings.NewReplacer(
		"<b>", "", "</b>",
		`<span foreground='#30d158'>`, "",
		`<span foreground='#ff9f0a'>`, "",
		`<span foreground='#8e8e93'>`, "",
		"</span>", "",
	)
	return r.Replace(s)
}

func firstOf(names ...string) string {
	for _, name := range names {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}
