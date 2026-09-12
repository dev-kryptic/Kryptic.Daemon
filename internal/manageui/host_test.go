package manageui

import "testing"

func TestHostLabelCloud(t *testing.T) {
	if HostLabel("https://daemon.kryptic.dev") != CloudName {
		t.Fatal(HostLabel("https://daemon.kryptic.dev"))
	}
	if HostLabel("https://daemon.kryptic.dev/") != CloudName {
		t.Fatal(HostLabel("https://daemon.kryptic.dev/"))
	}
	if HostLabel("https://daemon.work.internal") != "https://daemon.work.internal" {
		t.Fatal(HostLabel("https://daemon.work.internal"))
	}
}
