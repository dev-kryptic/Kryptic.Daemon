//go:build !windows

package singleinstance

import "testing"

// flock is per open file description, so a second Acquire in the same
// process contends exactly like a second process would.
func TestAcquireBlocksSecondInstance(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("TMPDIR", t.TempDir())

	release, ok := Acquire("kryptic-tray-test")
	if !ok {
		t.Fatal("first Acquire failed")
	}

	if _, ok := Acquire("kryptic-tray-test"); ok {
		t.Fatal("second Acquire succeeded while the first lock is held")
	}

	if _, ok := Acquire("kryptic-tray-test-other"); !ok {
		t.Fatal("a different name must not contend")
	}

	release()
	release2, ok := Acquire("kryptic-tray-test")
	if !ok {
		t.Fatal("Acquire after release failed")
	}
	release2()
}
