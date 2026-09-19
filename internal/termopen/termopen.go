package termopen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const ConnectorName = "local"

// Connector opens a visible terminal running `kryptic connector run --name local`
// so lease mint/revoke logs stay on screen. The session is the signed-in daemon.
func Connector() error {
	bin, err := krypticBinary()
	if err != nil {
		return err
	}
	return openTerminal(bin, []string{"connector", "run", "--name", ConnectorName})
}

func krypticBinary() (string, error) {
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		dir := filepath.Dir(exe)
		names := []string{"kryptic"}
		if runtime.GOOS == "windows" {
			names = []string{"kryptic.exe", "kryptic"}
		}
		for _, name := range names {
			sibling := filepath.Join(dir, name)
			if info, err := os.Stat(sibling); err == nil && !info.IsDir() {
				return sibling, nil
			}
		}
	}
	path, err := exec.LookPath("kryptic")
	if err != nil {
		return "", fmt.Errorf("kryptic is not on PATH")
	}
	return path, nil
}

func shellJoin(parts []string) string {
	out := make([]string, len(parts))
	for i, part := range parts {
		out[i] = "'" + strings.ReplaceAll(part, "'", `'"'"'`) + "'"
	}
	return strings.Join(out, " ")
}

func quoteWindows(s string) string {
	if !strings.ContainsAny(s, " \t\"") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
