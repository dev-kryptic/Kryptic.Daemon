package update

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/dev-kryptic/daemon/internal/server"
)

// ErrAvailable means --check found a newer release. The CLI maps it to exit 2.
var ErrAvailable = errors.New("update available")

// CheckResult is the latest GitHub release compared to the running binary.
type CheckResult struct {
	Current string
	Latest  string
	Newer   bool
	Release *release
}

func githubClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}

func userAgent() string {
	return "kryptic/" + server.Version + " (" + runtime.GOOS + "; " + runtime.GOARCH + ")"
}

// Check looks up the latest GitHub release. It does not download artifacts.
func Check(currentVersion string) (*CheckResult, error) {
	httpClient := githubClient()
	latest, err := fetchLatest(httpClient)
	if err != nil {
		return nil, err
	}
	latestVersion := strings.TrimPrefix(latest.TagName, "v")
	return &CheckResult{
		Current: currentVersion,
		Latest:  latestVersion,
		Newer:   latestVersion != "" && latestVersion != currentVersion,
		Release: latest,
	}, nil
}

// PrintCheck writes a one-line summary. Returns ErrAvailable when a newer
// release exists so callers can use exit status 2.
func PrintCheck(currentVersion string) error {
	result, err := Check(currentVersion)
	if err != nil {
		return err
	}
	if !result.Newer {
		fmt.Printf("kryptic %s is already the latest version.\n", result.Current)
		return nil
	}
	fmt.Printf("kryptic %s -> %s available\n", result.Current, result.Latest)
	return ErrAvailable
}

func (r *CheckResult) assetURL(name string) string {
	if r == nil || r.Release == nil {
		return ""
	}
	for _, asset := range r.Release.Assets {
		if asset.Name == name {
			return asset.BrowserDownloadURL
		}
	}
	return ""
}
