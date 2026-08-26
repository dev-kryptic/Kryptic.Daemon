package update

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestPreferInstallerLinuxNeverOpensDeb(t *testing.T) {
	if preferInstaller("linux", "/usr/bin/kryptic-tray") {
		t.Fatal("linux packaged installs should replace binaries in place")
	}
	if preferInstaller("linux", "/usr/bin/kryptic") {
		t.Fatal("linux packaged installs should replace binaries in place")
	}
}

func TestPreferInstallerKeepsMacAndWindows(t *testing.T) {
	if !preferInstaller("darwin", "/Applications/Kryptic.app/Contents/MacOS/Kryptic") {
		t.Fatal("macOS app should still open the pkg")
	}
	if !preferInstaller("windows", `C:\Users\x\AppData\Local\Kryptic\kryptic-tray.exe`) {
		t.Fatal("Windows tray should still open the setup exe")
	}
}

func TestJobsForLinuxUpdatesTrayAndCLI(t *testing.T) {
	dir := t.TempDir()
	tray := filepath.Join(dir, "kryptic-tray")
	cli := filepath.Join(dir, "kryptic")
	if err := os.WriteFile(tray, []byte("tray"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cli, []byte("cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	jobs := jobsFor("linux", "amd64", tray)
	if len(jobs) != 2 {
		t.Fatalf("got %d jobs, want 2: %+v", len(jobs), jobs)
	}
	if jobs[0].asset != "kryptic-tray_linux_amd64" || jobs[0].dest != tray {
		t.Fatalf("tray job: %+v", jobs[0])
	}
	if jobs[1].asset != "kryptic_linux_amd64" || jobs[1].dest != cli {
		t.Fatalf("cli job: %+v", jobs[1])
	}
}

func TestJobsForNonLinuxIsCLIOnly(t *testing.T) {
	jobs := jobsFor("darwin", "arm64", "/Applications/Kryptic.app/Contents/MacOS/kryptic")
	if len(jobs) != 1 || jobs[0].asset != "kryptic_darwin_arm64" {
		t.Fatalf("got %+v", jobs)
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
