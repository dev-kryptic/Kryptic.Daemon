// Package about is the About Kryptic panel shared by the macOS menu-bar app
// copy and the Windows/Linux tray. Same title, tagline, blurb, and website
// on every platform.
package about

import (
	"fmt"

	"github.com/dev-kryptic/daemon/internal/login"
	"github.com/dev-kryptic/daemon/internal/server"
)

const (
	WindowTitle  = "About Kryptic"
	AppName      = "Kryptic"
	Tagline      = "Zero-friction secrets management"
	Blurb        = "Authenticate once. Every project on this machine works. No prefix commands, no .env files."
	WebsiteLabel = "kryptic.dev"
	WebsiteURL   = "https://kryptic.dev"
	GitHubURL    = "https://github.com/dev-kryptic"
	DocsURL      = "https://docs.kryptic.dev"
)

// VersionLine matches the macOS About panel, using the same ldflags-stamped
// daemon version the CLI reports.
func VersionLine() string {
	return fmt.Sprintf("Version %s", server.Version)
}

// Show opens the About panel. A second call brings the existing panel forward
// instead of stacking another one. It never blocks the caller.
func Show() {
	go show()
}

func OpenWebsite() {
	login.OpenBrowser(WebsiteURL)
}

func OpenGitHub() {
	login.OpenBrowser(GitHubURL)
}

func OpenDocs() {
	login.OpenBrowser(DocsURL)
}
