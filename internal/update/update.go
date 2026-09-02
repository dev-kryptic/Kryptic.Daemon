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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dev-kryptic/daemon/internal/applog"
	"github.com/dev-kryptic/daemon/internal/pidfile"
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

type updateJob struct {
	asset string
	dest  string
}

type installFile struct {
	dest string
	data []byte
}

// Run updates the current executable to the latest release. currentVersion is
// compared against the release tag so an up-to-date install is a no-op.
func Run(currentVersion string) error {
	return run(currentVersion, cliReporter())
}

func run(currentVersion string, r Reporter) error {
	if r == nil {
		r = func(int, string) {}
	}
	r(0, "Checking for updates…")

	httpClient := githubClient()
	result, err := Check(currentVersion)
	if err != nil {
		applog.Error("cli", "update.check", err, "result=error")
		return err
	}
	if !result.Newer {
		applog.Event("cli", "update.check", "result=current")
		fmt.Printf("kryptic %s is already the latest version.\n", currentVersion)
		return nil
	}

	jobs, err := updateJobs(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	checksumsURL := result.assetURL("checksums.txt")
	if checksumsURL == "" {
		return fmt.Errorf("release %s has no checksums.txt - refusing to update unverified", result.Release.TagName)
	}
	if err := requireGitHubAsset(checksumsURL); err != nil {
		return err
	}
	for _, job := range jobs {
		if result.assetURL(job.asset) == "" {
			return fmt.Errorf("release %s has no binary %s", result.Release.TagName, job.asset)
		}
		if err := requireGitHubAsset(result.assetURL(job.asset)); err != nil {
			return err
		}
	}

	fmt.Printf("updating kryptic %s -> %s…\n", currentVersion, result.Latest)

	checksums, err := downloadProgress(httpClient, checksumsURL, 2, 8, r, "Downloading checksums…")
	if err != nil {
		return err
	}

	payloads := make([][]byte, len(jobs))
	span := 80
	if n := len(jobs); n > 0 {
		span = 80 / n
	}
	for i, job := range jobs {
		from := 8 + i*span
		to := from + span
		r(from, "Downloading "+job.asset+"…")
		payload, err := downloadProgress(httpClient, result.assetURL(job.asset), from, to, r, "Downloading update…")
		if err != nil {
			return err
		}
		if err := verify(payload, string(checksums), job.asset); err != nil {
			return err
		}
		payloads[i] = payload
	}

	r(90, "Installing…")
	files := make([]installFile, len(jobs))
	for i, job := range jobs {
		files[i] = installFile{dest: job.dest, data: payloads[i]}
	}
	if err := installFiles(files); err != nil {
		return err
	}

	r(96, "Restarting…")
	RestartDaemon()
	r(100, "Updated")
	applog.Event("cli", "update.apply", "result=ok")
	fmt.Printf("updated to kryptic %s. Existing sign-in was kept.\n", result.Latest)
	return nil
}

func binaryAssetName() string {
	return cliAssetName(runtime.GOOS, runtime.GOARCH)
}

func cliAssetName(goos, goarch string) string {
	name := fmt.Sprintf("kryptic_%s_%s", goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

func trayAssetName(goos, goarch string) string {
	name := fmt.Sprintf("kryptic-tray_%s_%s", goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

func isTrayExecutable(name string) bool {
	n := strings.ToLower(filepath.Base(name))
	n = strings.TrimSuffix(n, ".exe")
	return strings.Contains(n, "tray")
}

func updateJobs(goos, goarch string) ([]updateJob, error) {
	exe, err := currentExecutable()
	if err != nil {
		return nil, err
	}
	return jobsFor(goos, goarch, exe), nil
}

func jobsFor(goos, goarch, exe string) []updateJob {
	dir := filepath.Dir(exe)
	cliName := cliAssetName(goos, goarch)
	if goos != "linux" {
		return []updateJob{{asset: cliName, dest: exe}}
	}
	trayName := trayAssetName(goos, goarch)
	if isTrayExecutable(exe) {
		jobs := []updateJob{{asset: trayName, dest: exe}}
		sibling := filepath.Join(dir, "kryptic")
		if fileExists(sibling) {
			jobs = append(jobs, updateJob{asset: cliName, dest: sibling})
		}
		return jobs
	}
	jobs := []updateJob{{asset: cliName, dest: exe}}
	sibling := filepath.Join(dir, "kryptic-tray")
	if fileExists(sibling) {
		jobs = append(jobs, updateJob{asset: trayName, dest: sibling})
	}
	return jobs
}

func currentExecutable() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	// After an in-place update, /proc/self/exe on Linux points at the renamed
	// (and by then deleted) previous binary: "/path/kryptic.old (deleted)".
	// Map it back to the installed path so post-update restarts work.
	executable = strings.TrimSuffix(executable, " (deleted)")
	executable = strings.TrimSuffix(executable, ".old")
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	return executable, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func cliReporter() Reporter {
	fi, err := os.Stderr.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return func(int, string) {}
	}
	return func(percent int, message string) {
		fmt.Fprintf(os.Stderr, "\r%s (%d%%)\033[K", message, percent)
		if percent >= 100 {
			fmt.Fprintln(os.Stderr)
		}
	}
}

func fetchLatest(client *http.Client) (*release, error) {
	request, err := http.NewRequest(http.MethodGet, releasesURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", userAgent())
	request.Header.Set("Accept", "application/vnd.github+json")

	response, err := client.Do(request)
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

func download(client *http.Client, rawURL string) ([]byte, error) {
	return downloadProgress(client, rawURL, 0, 0, nil, "")
}

func downloadProgress(client *http.Client, rawURL string, fromPct, toPct int, r Reporter, message string) ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", userAgent())
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download of %s returned %s", rawURL, response.Status)
	}
	if r != nil && message != "" {
		r(fromPct, message)
	}
	total := response.ContentLength
	var buf []byte
	if total > 0 {
		buf = make([]byte, 0, total)
	}
	tmp := make([]byte, 32*1024)
	var copied int64
	span := toPct - fromPct
	for {
		n, readErr := response.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			copied += int64(n)
			if r != nil && total > 0 && span > 0 {
				pct := fromPct + int(copied*int64(span)/total)
				if pct > toPct {
					pct = toPct
				}
				r(pct, message)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	if r != nil && toPct > 0 {
		r(toPct, message)
	}
	return buf, nil
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
	executable, err := currentExecutable()
	if err != nil {
		return err
	}
	return replaceAt(executable, binary)
}

func replaceAt(dest string, binary []byte) error {
	staging := dest + ".new"
	if err := os.WriteFile(staging, binary, 0o755); err != nil {
		return err
	}

	old := dest + ".old"
	_ = os.Remove(old) // leftover from a previous update
	if err := os.Rename(dest, old); err != nil {
		_ = os.Remove(staging)
		return err
	}
	if err := os.Rename(staging, dest); err != nil {
		// Put the original back - never leave the user without a binary.
		if restoreErr := os.Rename(old, dest); restoreErr != nil {
			return errors.Join(err, restoreErr)
		}
		return err
	}
	_ = os.Remove(old) // fails on Windows while the old binary runs; harmless leftover
	return nil
}

func installFiles(files []installFile) error {
	var elevated [][2]string
	for _, file := range files {
		if err := replaceAt(file.dest, file.data); err == nil {
			continue
		} else if !permissionDenied(err) {
			return err
		}
		dir := filepath.Join(fallbackTempDir(), "Kryptic", "updates")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		staging := filepath.Join(dir, filepath.Base(file.dest)+".new")
		if err := os.WriteFile(staging, file.data, 0o755); err != nil {
			return err
		}
		elevated = append(elevated, [2]string{staging, file.dest})
	}
	if len(elevated) == 0 {
		return nil
	}
	err := privilegedInstall(elevated)
	for _, pair := range elevated {
		_ = os.Remove(pair[0])
	}
	return err
}

func permissionDenied(err error) bool {
	if err == nil {
		return false
	}
	if os.IsPermission(err) {
		return true
	}
	return errors.Is(err, os.ErrPermission)
}

// RestartDaemon stops the previous process (without logging out) and starts
// the newly installed binary. systemd/LaunchAgent are preferred when present.
func RestartDaemon() {
	if pid, err := pidfile.Read(); err == nil && pid == os.Getpid() {
		// In-process tray: do not kill ourselves. The caller re-execs.
		return
	}
	_ = pidfile.StopRunning()

	switch runtime.GOOS {
	case "linux":
		if err := exec.Command("systemctl", "--user", "restart", "kryptic-daemon").Run(); err == nil {
			return
		}
	case "darwin":
		spec := fmt.Sprintf("gui/%d/dev.kryptic.daemon", os.Getuid())
		if err := exec.Command("launchctl", "kickstart", "-k", spec).Run(); err == nil {
			return
		}
	}

	executable, err := currentExecutable()
	if err != nil {
		return
	}
	base := filepath.Base(executable)
	if strings.Contains(strings.ToLower(base), "tray") {
		sibling := filepath.Join(filepath.Dir(executable), "kryptic")
		if runtime.GOOS == "windows" {
			sibling += ".exe"
		}
		if info, err := os.Stat(sibling); err == nil && !info.IsDir() {
			executable = sibling
		} else {
			return
		}
	}
	cmd := exec.Command(executable, "start")
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Start()
}
