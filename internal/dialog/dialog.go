// Package dialog is the small native prompt layer used by the Windows and
// Linux tray (info, yes/no, text entry, progress, folder picker). macOS uses
// SwiftUI/AppKit instead.
package dialog

import "github.com/dev-kryptic/daemon/internal/config"

const (
	serverTitle   = "Server URI"
	serverMessage = "The Daemon BFF for this profile. Cloud and self-host can live side by side. Changing this URL signs this profile out only."

	newAccountTitle   = "New account server"
	newAccountMessage = "Which Daemon BFF should this account use? Use the hosted URL for cloud, or your company's self-hosted URI."

	envOverrideMessage = "KRYPTIC_API is set in the environment and overrides every profile's URL."

	cloudButton  = "Kryptic Cloud"
	saveButton   = "Save"
	cancelButton = "Cancel"

	changeServerTitle   = "Change this profile's server?"
	changeServerMessage = "This signs this profile out of the previous server. Other profiles keep their URL and session."
	changeServerAccept  = "Change Server"
)

// PromptServerURI is the Windows/Linux counterpart of the macOS Server URI
// sheet: Kryptic Cloud, Save, Cancel, then the same change-server confirm.
func PromptServerURI(current string) (string, bool) {
	return promptServer(serverTitle, serverMessage, current, true)
}

// PromptNewAccountServer asks which host a new profile should use. Same three
// buttons as PromptServerURI, without the sign-out confirm.
func PromptNewAccountServer(current string) (string, bool) {
	return promptServer(newAccountTitle, newAccountMessage, current, false)
}

func promptServer(title, message, current string, confirmChange bool) (string, bool) {
	if config.EnvOverrides() {
		Info("Kryptic", envOverrideMessage)
		return "", false
	}
	field, extra, ok := PromptExtra(title, message, current, cloudButton, saveButton, cancelButton)
	if !ok {
		return "", false
	}
	var next string
	if extra {
		next = config.DefaultAPI
	} else {
		normalized, err := config.NormalizeAPI(field)
		if err != nil {
			Info("Kryptic", err.Error())
			return "", false
		}
		next = normalized
	}
	currentNorm, err := config.NormalizeAPI(current)
	if err != nil {
		currentNorm = current
	}
	if confirmChange && next == currentNorm {
		return "", false
	}
	if !confirmChange {
		return next, true
	}
	if !Ask(changeServerTitle, changeServerMessage, changeServerAccept, cancelButton) {
		return "", false
	}
	return next, true
}

// Progress is a determinate 0-100 window. Close it when the work finishes.
// Canceled is closed when the user hits Cancel or closes the window. Close
// after a successful run does not treat the work as cancelled.
type Progress interface {
	Set(percent int, message string)
	Close()
	Canceled() <-chan struct{}
}

type nopProgress struct{}

func (nopProgress) Set(int, string)           {}
func (nopProgress) Close()                    {}
func (nopProgress) Canceled() <-chan struct{} { return nil }
