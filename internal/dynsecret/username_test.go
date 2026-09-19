package dynsecret

import "testing"

func TestRenderUsernameTruncatesIdentity(t *testing.T) {
	rendered := RenderUsername("{{truncate identity.name 4}}_{{random 6}}", "Ada Lovelace", "APP_DB", "postgres")
	if !hasPrefix(rendered, "AdaL_") {
		t.Fatalf("got %q", rendered)
	}
	if len(rendered) != 11 {
		t.Fatalf("len %d want 11 (%q)", len(rendered), rendered)
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
