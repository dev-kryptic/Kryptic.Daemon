package manageui

import "testing"

func TestActionRowsCoverEveryCommand(t *testing.T) {
	snap := Snapshot{
		Authenticated: true,
		UpdateTitle:   "Check for Updates",
		Profiles: []Profile{
			{ID: "work", Email: "a@b.c", Organization: "Acme", Active: true, SignedIn: true},
			{ID: "home", Email: "me@home", Active: false, SignedIn: true},
		},
	}
	got := map[string]bool{}
	for _, row := range ActionRows(snap) {
		got[row.Name] = true
	}
	for _, name := range []string{
		"signOut", "addAccount", "switchProfile", "deleteProfile",
		"flush", "scan", "update", "serverURI",
		"github", "docs", "logs", "about", "quit",
	} {
		if !got[name] {
			t.Fatalf("missing action %s", name)
		}
	}
	if got["signIn"] || got["cancelLogin"] {
		t.Fatal("signed-in snapshot should not offer sign-in")
	}
}

func TestProfileTitle(t *testing.T) {
	p := Profile{ID: "default", Email: "a@b.c", Organization: "Org", Active: true, SignedIn: true}
	if p.Title() != "✓ a@b.c · Org" {
		t.Fatalf("got %q", p.Title())
	}
}

func TestEncodePanelSnapUsesHostLabel(t *testing.T) {
	got := encodePanelSnap(Snapshot{
		API:     "https://daemon.kryptic.dev",
		Version: "1.2.3",
		Profiles: []Profile{
			{ID: "p", Email: "a@b.c", API: "https://daemon.kryptic.dev", Active: true, SignedIn: true},
		},
	})
	if got.APILabel != CloudName {
		t.Fatalf("api label %q", got.APILabel)
	}
	if got.Version != "1.2.3" || len(got.Profiles) != 1 || got.Profiles[0].APILabel != CloudName {
		t.Fatalf("%+v", got)
	}
}

func TestConnectionLabel(t *testing.T) {
	if ConnectionLabel("connected", "") != "Connected" {
		t.Fatal(ConnectionLabel("connected", ""))
	}
	if ConnectionLabel("awaiting_approval", "") != "Awaiting approval" {
		t.Fatal(ConnectionLabel("awaiting_approval", ""))
	}
}
