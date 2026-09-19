package server

import (
	"net/http/httptest"
	"testing"
)

func TestBrowserOriginAllowed(t *testing.T) {
	t.Setenv("KRYPTIC_DASHBOARD_ORIGINS", "https://vault.example.com")

	const hosted = "https://daemon.kryptic.dev"
	cases := []struct {
		origin string
		base   string
		want   bool
	}{
		{"http://localhost:5173", hosted, true},
		{"http://127.0.0.1:5173", hosted, true},
		{"https://app.kryptic.dev", hosted, true},
		{"https://vault.example.com", hosted, true},
		// Only the dashboard, not every subdomain: a compromised marketing
		// or docs host must not be able to drive mints. The hosted apex
		// never qualifies through the self-host derivation either.
		{"https://docs.kryptic.dev", hosted, false},
		{"https://evil.kryptic.dev", hosted, false},
		{"https://kryptic.dev", hosted, false},
		// No plain http off the loopback, even for the real dashboard host.
		{"http://app.kryptic.dev", hosted, false},
		{"https://evil.example", hosted, false},
		{"", hosted, false},

		// Self-hosted: the signed-in Daemon BFF URL implies the dashboard.
		// Reference kit: dashboard on the parent of the daemon. subdomain.
		{"https://kryptic.acme.com", "https://daemon.kryptic.acme.com", true},
		{"https://daemon.kryptic.acme.com", "https://daemon.kryptic.acme.com", true},
		{"https://other.acme.com", "https://daemon.kryptic.acme.com", false},
		// Single-host installs (dashboard and BFF behind one domain).
		{"https://secrets.internal.corp", "https://secrets.internal.corp:8443", true},
		// http dashboard is only trusted when the BFF itself is http (labs).
		{"http://kryptic.lab.local", "http://daemon.kryptic.lab.local", true},
		{"http://kryptic.acme.com", "https://daemon.kryptic.acme.com", false},
	}
	for _, tc := range cases {
		if got := browserOriginAllowed(tc.origin, tc.base); got != tc.want {
			t.Fatalf("browserOriginAllowed(%q, %q)=%v want %v", tc.origin, tc.base, got, tc.want)
		}
	}
}

// A random website must not be able to read /status (daemon presence,
// version, signed-in state) from a victim's browser.
func TestBrowserCORSDoesNotReflectUnknownOrigins(t *testing.T) {
	const hosted = "https://daemon.kryptic.dev"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/status", nil)
	request.Header.Set("Origin", "https://evil.example")
	writeBrowserCORS(recorder, request, hosted)
	if header := recorder.Header().Get("Access-Control-Allow-Origin"); header != "" {
		t.Fatalf("reflected disallowed origin: %q", header)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest("GET", "/status", nil)
	request.Header.Set("Origin", "https://app.kryptic.dev")
	writeBrowserCORS(recorder, request, hosted)
	if header := recorder.Header().Get("Access-Control-Allow-Origin"); header != "https://app.kryptic.dev" {
		t.Fatalf("dashboard origin not reflected: %q", header)
	}
}
