package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// PreferInstaller is true when this process came from a signed macOS app,
// Windows setup, or Linux .deb. Those installs should take the pkg/exe/deb
// rather than replacing a single binary in place.
func PreferInstaller() bool {
	executable, err := os.Executable()
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	switch runtime.GOOS {
	case "darwin":
		return strings.Contains(executable, ".app/Contents/MacOS")
	case "windows":
		lower := strings.ToLower(executable)
		return strings.Contains(lower, `\kryptic\`) || strings.HasSuffix(lower, `kryptic-tray.exe`)
	default:
		return strings.HasPrefix(executable, "/usr/")
	}
}

// Apply picks the installer when this looks like a packaged install, otherwise
// replaces the running binary in place.
func Apply(currentVersion string) error {
	if PreferInstaller() {
		return RunInstaller(currentVersion)
	}
	return Run(currentVersion)
}

// RunInstaller downloads the signed platform installer, verifies it against
// checksums.txt, and opens it. The installer stops the old app and keeps the
// saved sign-in.
func RunInstaller(currentVersion string) error {
	result, err := Check(currentVersion)
	if err != nil {
		return err
	}
	if !result.Newer {
		fmt.Printf("kryptic %s is already the latest version.\n", currentVersion)
		return nil
	}

	assetName, err := installerAssetName(runtime.GOOS, runtime.GOARCH, result.Latest)
	if err != nil {
		return Run(currentVersion)
	}
	installerURL := result.assetURL(assetName)
	checksumsURL := result.assetURL("checksums.txt")
	if installerURL == "" {
		return Run(currentVersion)
	}
	if checksumsURL == "" {
		return fmt.Errorf("release %s has no checksums.txt - refusing to update unverified", result.Release.TagName)
	}
	if err := requireGitHubAsset(installerURL); err != nil {
		return err
	}
	if err := requireGitHubAsset(checksumsURL); err != nil {
		return err
	}

	fmt.Printf("downloading Kryptic %s installer…\n", result.Latest)

	httpClient := githubClient()
	payload, err := download(httpClient, installerURL)
	if err != nil {
		return err
	}
	checksums, err := download(httpClient, checksumsURL)
	if err != nil {
		return err
	}
	if err := verify(payload, string(checksums), assetName); err != nil {
		return err
	}

	path, err := installerDest(assetName)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, payload, 0o755); err != nil {
		return err
	}

	if err := openInstaller(path); err != nil {
		return err
	}
	fmt.Printf("opened %s. Finish the installer to complete the update. Existing sign-in is kept.\n", assetName)
	return nil
}

func installerDest(assetName string) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil || base == "" || !dirExists(base) {
		base = fallbackTempDir()
	}
	dir := filepath.Join(base, "Kryptic", "updates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		dir = fallbackTempDir()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	return filepath.Join(dir, assetName), nil
}

func fallbackTempDir() string {
	if runtime.GOOS == "windows" {
		for _, key := range []string{"LOCALAPPDATA", "TEMP", "TMP"} {
			if v := os.Getenv(key); v != "" && dirExists(v) {
				return filepath.Join(v, "Kryptic", "updates")
			}
		}
		return filepath.Join(`C:\Windows\Temp`, "Kryptic", "updates")
	}
	if dirExists("/tmp") {
		return "/tmp"
	}
	return "/var/tmp"
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func installerAssetName(goos, goarch, version string) (string, error) {
	switch goos {
	case "darwin":
		label := "intel"
		if goarch == "arm64" {
			label = "apple-silicon"
		}
		return fmt.Sprintf("Kryptic-%s-macos-%s.pkg", version, label), nil
	case "windows":
		return fmt.Sprintf("Kryptic-Setup-%s-windows-amd64.exe", version), nil
	case "linux":
		if goarch != "amd64" && goarch != "arm64" {
			return "", fmt.Errorf("no Linux installer for %s", goarch)
		}
		return fmt.Sprintf("kryptic_%s_%s.deb", version, goarch), nil
	default:
		return "", fmt.Errorf("no installer for %s/%s", goos, goarch)
	}
}

func openInstaller(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", path).Start()
	case "windows":
		return openWindowsInstaller(path)
	default:
		return exec.Command("xdg-open", path).Start()
	}
}
