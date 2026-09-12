//go:build linux

package manageui

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var showing sync.Mutex

// Show opens a native Linux dialog (yad, then zenity) with the same actions
// as Open Kryptic on macOS and Windows.
func Show(h Handlers) {
	if !showing.TryLock() {
		return
	}
	beginSession()
	go func() {
		defer showing.Unlock()
		defer endSession()
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
