// Package update implements `kryptic update`: fetch the latest GitHub release,
// verify the platform binary against the release's checksums.txt, and replace
// the running executable in place. Pure stdlib.
package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const releasesURL = "https://api.github.com/repos/dev-kryptic/Kryptic.Daemon/releases/latest"

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type release struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

// Run updates the current executable to the latest release. currentVersion is
// compared against the release tag so an up-to-date install is a no-op.
func Run(currentVersion string) error {
	httpClient := &http.Client{Timeout: 60 * time.Second}

	latest, err := fetchLatest(httpClient)
	if err != nil {
		return err
	}

	latestVersion := strings.TrimPrefix(latest.TagName, "v")
	if latestVersion == currentVersion {
		fmt.Printf("kryptic %s is already the latest version.\n", currentVersion)
		return nil
	}

	assetName := fmt.Sprintf("kryptic_%s_%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName += ".exe"
	}

	binaryURL, checksumsURL := "", ""
	for _, asset := range latest.Assets {
		switch asset.Name {
		case assetName:
			binaryURL = asset.BrowserDownloadURL
		case "checksums.txt":
			checksumsURL = asset.BrowserDownloadURL
		}
	}
	if binaryURL == "" {
		return fmt.Errorf("release %s has no binary for %s/%s", latest.TagName, runtime.GOOS, runtime.GOARCH)
	}
	if checksumsURL == "" {
		return fmt.Errorf("release %s has no checksums.txt - refusing to update unverified", latest.TagName)
	}

	// The release metadata is JSON we do not sign, so treat the asset URLs as
	// untrusted: only fetch over HTTPS from GitHub's own hosts. This stops a
	// tampered release response from redirecting the download to an attacker
	// origin. (Integrity against the release itself is checked in verify.)
	if err := requireGitHubAsset(binaryURL); err != nil {
		return err
	}
	if err := requireGitHubAsset(checksumsURL); err != nil {
		return err
	}

	fmt.Printf("updating kryptic %s -> %s…\n", currentVersion, latestVersion)

	binary, err := download(httpClient, binaryURL)
	if err != nil {
		return err
	}
	checksums, err := download(httpClient, checksumsURL)
	if err != nil {
		return err
	}

	if err := verify(binary, string(checksums), assetName); err != nil {
		return err
	}

	if err := replaceExecutable(binary); err != nil {
		return err
	}

	fmt.Printf("updated to kryptic %s. Restart the daemon (`kryptic stop && kryptic start`) to run it.\n", latestVersion)
	return nil
}

func fetchLatest(client *http.Client) (*release, error) {
	response, err := client.Get(releasesURL)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub releases returned %s", response.Status)
	}

	var latest release
	if err := json.NewDecoder(response.Body).Decode(&latest); err != nil {
		return nil, err
	}
	return &latest, nil
}

// requireGitHubAsset rejects any download URL that is not HTTPS on a GitHub
// host. GitHub serves release assets from github.com (which redirects to
// *.githubusercontent.com); the redirect target is validated by the HTTP
// client, so checking the advertised URL host is enough.
func requireGitHubAsset(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid asset URL %q: %w", raw, err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("refusing non-HTTPS asset URL %q", raw)
	}
	host := parsed.Hostname()
	if host != "github.com" &&
		!strings.HasSuffix(host, ".github.com") &&
		!strings.HasSuffix(host, ".githubusercontent.com") {
		return fmt.Errorf("refusing asset URL from unexpected host %q", host)
	}
	return nil
}

func download(client *http.Client, url string) ([]byte, error) {
	response, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download of %s returned %s", url, response.Status)
	}
	return io.ReadAll(response.Body)
}

// verify checks the downloaded binary against its line in checksums.txt
// (the standard "<sha256>  <filename>" format).
func verify(binary []byte, checksums, assetName string) error {
	sum := sha256.Sum256(binary)
	actual := hex.EncodeToString(sum[:])

	for _, line := range strings.Split(checksums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == assetName {
			if fields[0] == actual {
				return nil
			}
			return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", assetName, fields[0], actual)
		}
	}
	return fmt.Errorf("checksums.txt has no entry for %s", assetName)
}

// replaceExecutable swaps the running binary using the rename dance that works
// on every OS (a running executable can be renamed, not overwritten).
func replaceExecutable(binary []byte) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}

	staging := executable + ".new"
	if err := os.WriteFile(staging, binary, 0o755); err != nil {
		return err
	}

	old := executable + ".old"
	_ = os.Remove(old) // leftover from a previous update
	if err := os.Rename(executable, old); err != nil {
		_ = os.Remove(staging)
		return err
	}
	if err := os.Rename(staging, executable); err != nil {
		// Put the original back - never leave the user without a binary.
		if restoreErr := os.Rename(old, executable); restoreErr != nil {
			return errors.Join(err, restoreErr)
		}
		return err
	}
	_ = os.Remove(old) // fails on Windows while the old binary runs; harmless leftover
	return nil
}
