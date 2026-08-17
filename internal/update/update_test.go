package update

import "testing"

func TestRequireGitHubAssetAccepts(t *testing.T) {
	valid := []string{
		"https://github.com/dev-kryptic/daemon/releases/download/v1.0.0/kryptic_linux_amd64",
		"https://objects.githubusercontent.com/github-production-release-asset/123/abc",
		"https://release-assets.githubusercontent.com/foo/bar",
	}
	for _, raw := range valid {
		if err := requireGitHubAsset(raw); err != nil {
			t.Errorf("requireGitHubAsset(%q) = %v, want nil", raw, err)
		}
	}
}

func TestRequireGitHubAssetRejects(t *testing.T) {
	invalid := []string{
		"http://github.com/dev-kryptic/daemon/releases/download/v1/kryptic",   // not HTTPS
		"https://evil.example.com/kryptic_linux_amd64",                        // wrong host
		"https://github.com.attacker.net/kryptic",                             // suffix trick
		"https://raw.githubusercontent.com.evil.net/kryptic",                  // suffix trick
		"ftp://github.com/kryptic",                                            // wrong scheme
	}
	for _, raw := range invalid {
		if err := requireGitHubAsset(raw); err == nil {
			t.Errorf("requireGitHubAsset(%q) = nil, want error", raw)
		}
	}
}
