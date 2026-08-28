package main

import "strings"

// execTarget maps the running process's executable path back to the path the
// update installed to. After the in-place rename dance (kryptic-tray ->
// kryptic-tray.old -> deleted), /proc/self/exe on Linux reads
// "/path/kryptic-tray.old (deleted)". Exec-ing that fails with ENOENT, which
// used to leave the old version running after an update.
func execTarget(exe string) string {
	exe = strings.TrimSuffix(exe, " (deleted)")
	exe = strings.TrimSuffix(exe, ".old")
	return exe
}
