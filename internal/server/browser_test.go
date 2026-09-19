package server

import "testing"

func TestBrowserOriginAllowed(t *testing.T) {
	t.Setenv("KRYPTIC_DASHBOARD_ORIGINS", "https://vault.example.com")

	cases := []struct {
		origin string
		want   bool
	}{
		{"http://localhost:5173", true},
		{"http://127.0.0.1:5173", true},
		{"https://app.kryptic.dev", true},
		{"https://docs.kryptic.dev", true},
		{"https://vault.example.com", true},
		{"https://evil.example", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := browserOriginAllowed(tc.origin); got != tc.want {
			t.Fatalf("browserOriginAllowed(%q)=%v want %v", tc.origin, got, tc.want)
		}
	}
}
