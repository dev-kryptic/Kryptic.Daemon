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
	fileOnlyMu.Lock()
	previous := fileOnly
	fileOnly = true
	fileOnlyMu.Unlock()
	t.Cleanup(func() {
		fileOnlyMu.Lock()
		fileOnly = previous
		fileOnlyMu.Unlock()
	})
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
	saved.ID = "default"
	if loaded != saved {
		t.Fatalf("loaded %+v, want %+v", loaded, saved)
	}
}

func TestKeysOnlySessionSurvivesLogoutShape(t *testing.T) {
	loaded, err := decodeSession(`{"devicePublicKey":"BPub","devicePrivateKey":"Priv"}`)
	if err != nil {
		t.Fatalf("keys-only session: %v", err)
	}
	if loaded.RefreshToken != "" || !HasDeviceKeys(loaded) || loaded.ID != "default" {
		t.Fatalf("unexpected session %+v", loaded)
	}
}

func TestLogoutKeepsKeysAndResetClearsFile(t *testing.T) {
	withTempConfigDir(t)

	full, err := json.Marshal(Session{
		RefreshToken:     "rt_secret_token",
		DevicePublicKey:  "BPub",
		DevicePrivateKey: "Priv",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := saveFile(string(full)); err != nil {
		t.Fatalf("saveFile: %v", err)
	}

	raw, err := loadFile()
	if err != nil {
		t.Fatalf("loadFile: %v", err)
	}
	session, err := decodeSession(raw)
	if err != nil {
		t.Fatalf("decodeSession: %v", err)
	}
	session.RefreshToken = ""
	kept, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("marshal keys-only: %v", err)
	}
	if err := saveFile(string(kept)); err != nil {
		t.Fatalf("save keys-only: %v", err)
	}

	afterLogout, err := decodeSession(mustLoadFile(t))
	if err != nil {
		t.Fatalf("after logout: %v", err)
	}
	if afterLogout.RefreshToken != "" || !HasDeviceKeys(afterLogout) {
		t.Fatalf("logout dropped keys: %+v", afterLogout)
	}

	path, err := filePath()
	if err != nil {
		t.Fatalf("filePath: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("reset wipe: %v", err)
	}
	if _, err := loadFile(); err != ErrNotLoggedIn {
		t.Fatalf("keys survived reset: %v", err)
	}
}

func mustLoadFile(t *testing.T) string {
	t.Helper()
	raw, err := loadFile()
	if err != nil {
		t.Fatalf("loadFile: %v", err)
	}
	return raw
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

func TestLegacySessionBecomesDefaultProfile(t *testing.T) {
	store, err := decodeStore(`{"refreshToken":"rt","devicePublicKey":"BPub","devicePrivateKey":"Priv"}`)
	if err != nil {
		t.Fatalf("decodeStore: %v", err)
	}
	if store.ActiveID != "default" || len(store.Profiles) != 1 {
		t.Fatalf("store %+v", store)
	}
	if store.Profiles[0].RefreshToken != "rt" {
		t.Fatalf("token %q", store.Profiles[0].RefreshToken)
	}
}

func TestTwoProfilesSwitchAndLogoutKeepsTheOther(t *testing.T) {
	withTempConfigDir(t)

	if err := SaveLoggedIn(Session{
		Email:            "me@personal.test",
		Organization:     "Personal",
		RefreshToken:     "rt-personal",
		DevicePublicKey:  "PubA",
		DevicePrivateKey: "PrivA",
	}, false); err != nil {
		t.Fatalf("first login: %v", err)
	}
	if err := SaveLoggedIn(Session{
		Email:            "me@work.test",
		Organization:     "Work",
		RefreshToken:     "rt-work",
		DevicePublicKey:  "PubB",
		DevicePrivateKey: "PrivB",
	}, true); err != nil {
		t.Fatalf("second login: %v", err)
	}

	active, err := LoadSession()
	if err != nil {
		t.Fatalf("active: %v", err)
	}
	if active.Email != "me@work.test" || active.RefreshToken != "rt-work" {
		t.Fatalf("wanted work active, got %+v", active)
	}

	if err := Switch("me@personal.test"); err != nil {
		t.Fatalf("switch: %v", err)
	}
	active, err = LoadSession()
	if err != nil {
		t.Fatalf("after switch: %v", err)
	}
	if active.Email != "me@personal.test" {
		t.Fatalf("wanted personal, got %+v", active)
	}

	if err := ClearRefresh(); err != nil {
		t.Fatalf("logout: %v", err)
	}
	store, err := LoadStore()
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	personal, ok := FindByIdentity(store, "me@personal.test", "Personal", "")
	if !ok || personal.SignedIn() || !personal.HasDeviceKeys() {
		t.Fatalf("personal after logout: %+v ok=%v", personal, ok)
	}
	work, ok := FindByIdentity(store, "me@work.test", "Work", "")
	if !ok || work.RefreshToken != "rt-work" {
		t.Fatalf("work should stay signed in: %+v ok=%v", work, ok)
	}
}

func TestSaveLoggedInMergesSameIdentity(t *testing.T) {
	withTempConfigDir(t)

	if err := SaveLoggedIn(Session{
		Email:            "me@work.test",
		Organization:     "Work",
		RefreshToken:     "rt-1",
		DevicePublicKey:  "PubKeep",
		DevicePrivateKey: "PrivKeep",
	}, false); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := SaveLoggedIn(Session{
		Email:            "me@work.test",
		Organization:     "Work",
		RefreshToken:     "rt-2",
		DevicePublicKey:  "PubNew",
		DevicePrivateKey: "PrivNew",
	}, true); err != nil {
		t.Fatalf("merge: %v", err)
	}

	store, err := LoadStore()
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if len(store.Profiles) != 1 {
		t.Fatalf("profiles %d", len(store.Profiles))
	}
	p := store.Profiles[0]
	if p.RefreshToken != "rt-2" || p.DevicePrivateKey != "PrivKeep" {
		t.Fatalf("merge kept wrong fields: %+v", p)
	}
}

func TestRemoveActiveLeavesTheOtherProfile(t *testing.T) {
	withTempConfigDir(t)

	if err := SaveLoggedIn(Session{
		Email: "a@test", Organization: "A", RefreshToken: "a",
		DevicePublicKey: "Pa", DevicePrivateKey: "Sa",
	}, false); err != nil {
		t.Fatal(err)
	}
	if err := SaveLoggedIn(Session{
		Email: "b@test", Organization: "B", RefreshToken: "b",
		DevicePublicKey: "Pb", DevicePrivateKey: "Sb",
	}, true); err != nil {
		t.Fatal(err)
	}
	if err := RemoveActive(); err != nil {
		t.Fatal(err)
	}
	store, err := LoadStore()
	if err != nil {
		t.Fatal(err)
	}
	if len(store.Profiles) != 1 || store.Profiles[0].Email != "a@test" {
		t.Fatalf("store %+v", store)
	}
}

func TestSameIdentityDifferentServersStaySeparate(t *testing.T) {
	withTempConfigDir(t)

	if err := SaveLoggedIn(Session{
		Email: "me@work.test", Organization: "Acme", RefreshToken: "cloud",
		DevicePublicKey: "Pc", DevicePrivateKey: "Sc",
		Config: ProfileConfig{API: "https://daemon.kryptic.dev"},
	}, false); err != nil {
		t.Fatal(err)
	}
	if err := SaveLoggedIn(Session{
		Email: "me@work.test", Organization: "Acme", RefreshToken: "self",
		DevicePublicKey: "Ps", DevicePrivateKey: "Ss",
		Config: ProfileConfig{API: "https://daemon.work.internal"},
	}, true); err != nil {
		t.Fatal(err)
	}

	store, err := LoadStore()
	if err != nil {
		t.Fatal(err)
	}
	if len(store.Profiles) != 2 {
		t.Fatalf("wanted 2 profiles, got %d", len(store.Profiles))
	}
	cloud, ok := FindByIdentity(store, "me@work.test", "Acme", "https://daemon.kryptic.dev")
	if !ok || cloud.RefreshToken != "cloud" || cloud.DevicePrivateKey != "Sc" {
		t.Fatalf("cloud %+v ok=%v", cloud, ok)
	}
	self, ok := FindByIdentity(store, "me@work.test", "Acme", "https://daemon.work.internal")
	if !ok || self.RefreshToken != "self" || self.DevicePrivateKey != "Ss" {
		t.Fatalf("self %+v ok=%v", self, ok)
	}
}

func TestSetActiveAPILeavesTheOtherProfile(t *testing.T) {
	withTempConfigDir(t)

	if err := SaveLoggedIn(Session{
		Email: "a@test", Organization: "A", RefreshToken: "ra",
		DevicePublicKey: "Pa", DevicePrivateKey: "Sa",
		Config: ProfileConfig{API: "https://daemon.kryptic.dev"},
	}, false); err != nil {
		t.Fatal(err)
	}
	if err := SaveLoggedIn(Session{
		Email: "b@test", Organization: "B", RefreshToken: "rb",
		DevicePublicKey: "Pb", DevicePrivateKey: "Sb",
		Config: ProfileConfig{API: "https://self.example"},
	}, true); err != nil {
		t.Fatal(err)
	}
	if err := SetActiveAPI("https://self.example/v2"); err != nil {
		t.Fatal(err)
	}
	store, err := LoadStore()
	if err != nil {
		t.Fatal(err)
	}
	work, ok := FindByIdentity(store, "b@test", "B", "https://self.example/v2")
	if !ok || work.SignedIn() {
		t.Fatalf("active should be signed out after URL change: %+v ok=%v", work, ok)
	}
	personal, ok := FindByIdentity(store, "a@test", "A", "https://daemon.kryptic.dev")
	if !ok || personal.RefreshToken != "ra" {
		t.Fatalf("other profile should keep its session: %+v ok=%v", personal, ok)
	}
}

func TestRemoveDeletesOnlyThatProfile(t *testing.T) {
	withTempConfigDir(t)

	if err := SaveLoggedIn(Session{
		Email: "keep@test", Organization: "K", RefreshToken: "k",
		DevicePublicKey: "Pk", DevicePrivateKey: "Sk",
	}, false); err != nil {
		t.Fatal(err)
	}
	if err := SaveLoggedIn(Session{
		Email: "gone@test", Organization: "G", RefreshToken: "g",
		DevicePublicKey: "Pg", DevicePrivateKey: "Sg",
	}, true); err != nil {
		t.Fatal(err)
	}
	if err := Remove("gone@test"); err != nil {
		t.Fatal(err)
	}
	store, err := LoadStore()
	if err != nil {
		t.Fatal(err)
	}
	if len(store.Profiles) != 1 || store.Profiles[0].Email != "keep@test" {
		t.Fatalf("store %+v", store)
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
