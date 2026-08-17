package authstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The platform stores (Keychain, Credential Manager, libsecret) need a real OS
// session, so tests cover the shared file fallback - the path every platform
// degrades to.

func withTempConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// os.UserConfigDir honors XDG_CONFIG_HOME on Linux and HOME elsewhere.
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	return dir
}

func TestFileFallbackRoundTrip(t *testing.T) {
	withTempConfigDir(t)

	if err := saveFile("rt_secret_token"); err != nil {
		t.Fatalf("saveFile: %v", err)
	}

	token, err := loadFile()
	if err != nil {
		t.Fatalf("loadFile: %v", err)
	}
	if token != "rt_secret_token" {
		t.Fatalf("loaded %q, want %q", token, "rt_secret_token")
	}
}

func TestFileFallbackPermissions(t *testing.T) {
	withTempConfigDir(t)

	if err := saveFile("tok"); err != nil {
		t.Fatalf("saveFile: %v", err)
	}
	path, err := filePath()
	if err != nil {
		t.Fatalf("filePath: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("session file permissions %o, want 600", perm)
	}
	if perm := mustStat(t, filepath.Dir(path)).Mode().Perm(); perm != 0o700 {
		t.Fatalf("config dir permissions %o, want 700", perm)
	}
}

func TestLoadWithoutSessionReturnsNotLoggedIn(t *testing.T) {
	withTempConfigDir(t)

	if _, err := loadFile(); err != ErrNotLoggedIn {
		t.Fatalf("err = %v, want ErrNotLoggedIn", err)
	}
}

// The encode/decode layer is tested directly so the tests never touch the real
// platform credential store of the machine running them.
func TestSessionRoundTripsDeviceKeys(t *testing.T) {
	withTempConfigDir(t)

	saved := Session{
		RefreshToken:     "rt_secret_token",
		DevicePublicKey:  "BPubKey",
		DevicePrivateKey: "PrivKey",
	}
	data, err := json.Marshal(saved)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := saveFile(string(data)); err != nil {
		t.Fatalf("saveFile: %v", err)
	}

	raw, err := loadFile()
	if err != nil {
		t.Fatalf("loadFile: %v", err)
	}
	loaded, err := decodeSession(raw)
	if err != nil {
		t.Fatalf("decodeSession: %v", err)
	}
	if loaded != saved {
		t.Fatalf("loaded %+v, want %+v", loaded, saved)
	}
}

func TestCorruptSessionReturnsNotLoggedIn(t *testing.T) {
	if _, err := decodeSession("not-json"); err != ErrNotLoggedIn {
		t.Fatalf("err = %v, want ErrNotLoggedIn", err)
	}
	if _, err := decodeSession(`{"devicePublicKey":"x"}`); err != ErrNotLoggedIn {
		t.Fatalf("missing refresh token: err = %v, want ErrNotLoggedIn", err)
	}
}

// Clear() itself is not exercised here: on a developer machine it would also
// delete the real Keychain/Credential Manager entry for a logged-in daemon.
func TestClearedFileReturnsNotLoggedIn(t *testing.T) {
	withTempConfigDir(t)

	if err := saveFile("tok"); err != nil {
		t.Fatalf("saveFile: %v", err)
	}
	path, err := filePath()
	if err != nil {
		t.Fatalf("filePath: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := loadFile(); err != ErrNotLoggedIn {
		t.Fatalf("token survived removal: err = %v", err)
	}
}

func mustStat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info
}
