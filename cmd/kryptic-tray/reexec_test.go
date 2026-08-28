package main

import "testing"

func TestExecTarget(t *testing.T) {
	cases := map[string]string{
		// /proc/self/exe after the update's rename dance removed the old file.
		"/home/u/.local/bin/kryptic-tray.old (deleted)": "/home/u/.local/bin/kryptic-tray",
		// Rename happened but the .old file was not deleted yet.
		"/usr/bin/kryptic-tray.old": "/usr/bin/kryptic-tray",
		// No update in flight: the path is untouched.
		"/usr/bin/kryptic-tray": "/usr/bin/kryptic-tray",
	}
	for in, want := range cases {
		if got := execTarget(in); got != want {
			t.Errorf("execTarget(%q) = %q, want %q", in, got, want)
		}
	}
}
