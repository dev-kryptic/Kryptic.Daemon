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
		"http://github.com/dev-kryptic/daemon/releases/download/v1/kryptic", // not HTTPS
		"https://evil.example.com/kryptic_linux_amd64",                      // wrong host
		"https://github.com.attacker.net/kryptic",                           // suffix trick
		"https://raw.githubusercontent.com.evil.net/kryptic",                // suffix trick
		"ftp://github.com/kryptic",                                          // wrong scheme
	}
	for _, raw := range invalid {
		if err := requireGitHubAsset(raw); err == nil {
			t.Errorf("requireGitHubAsset(%q) = nil, want error", raw)
		}
	}
}

func TestInstallerAssetName(t *testing.T) {
	cases := []struct {
		goos, goarch, version, want string
	}{
		{"darwin", "arm64", "0.0.8", "Kryptic-0.0.8-macos-apple-silicon.pkg"},
		{"darwin", "amd64", "0.0.8", "Kryptic-0.0.8-macos-intel.pkg"},
		{"windows", "amd64", "0.0.8", "Kryptic-Setup-0.0.8-windows-amd64.exe"},
		{"linux", "amd64", "0.0.8", "kryptic_0.0.8_amd64.deb"},
		{"linux", "arm64", "0.0.8", "kryptic_0.0.8_arm64.deb"},
	}
	for _, c := range cases {
		got, err := installerAssetName(c.goos, c.goarch, c.version)
		if err != nil {
			t.Fatalf("%s/%s: %v", c.goos, c.goarch, err)
		}
		if got != c.want {
			t.Fatalf("%s/%s: got %q want %q", c.goos, c.goarch, got, c.want)
		}
	}
}
