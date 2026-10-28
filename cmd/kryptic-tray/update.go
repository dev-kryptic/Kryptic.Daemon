package main

import (
	"fmt"
	"time"

	"fyne.io/systray"
	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/authstore"
	"github.com/dev-kryptic/daemon/internal/dialog"
	"github.com/dev-kryptic/daemon/internal/login"
	"github.com/dev-kryptic/daemon/internal/server"
	"github.com/dev-kryptic/daemon/internal/update"
)

func watchForUpdates(panel *panelState, item *systray.MenuItem) {
	time.Sleep(8 * time.Second)
	for {
		result, err := update.Check(server.Version)
		if err == nil && result.Newer {
			panel.setUpdateTitle("Update Available…")
			if item != nil {
				item.SetTitle("Update Available…")
			}
		}
		time.Sleep(12 * time.Hour)
	}
}

func runUpdateFlow(item *systray.MenuItem) {
	setUpdate := func(title string) {
		if item != nil {
			item.SetTitle(title)
		}
	}
	setUpdate("Checking for Updates…")
	result, err := update.Check(server.Version)
	setUpdate("Check for Updates")
	if err != nil {
		dialog.Info("Kryptic", "Could not check for updates: "+err.Error())
		return
	}
	if !result.Newer {
		dialog.Info("Kryptic", "Kryptic "+result.Current+" is already the latest version.")
		return
	}
	message := fmt.Sprintf("Version %s is available (you have %s). Update now?", result.Latest, result.Current)
	if !dialog.Confirm("Kryptic", message) {
		setUpdate("Update Available…")
		return
	}
	setUpdate("Updating…")
	var progress dialog.Progress
	if !update.PreferInstaller() {
		progress = dialog.OpenProgress("Kryptic", "Updating…")
		defer progress.Close()
	}
	err = update.ApplyWithProgress(server.Version, func(percent int, message string) {
		setUpdate(fmt.Sprintf("Updating… %d%%", percent))
		if progress != nil {
			progress.Set(percent, message)
		}
	})
	if progress != nil {
		progress.Close()
	}
	setUpdate("Check for Updates")
	if err != nil {
		dialog.Info("Kryptic", "Update failed: "+err.Error())
		setUpdate("Update Available…")
		return
	}
	if update.PreferInstaller() {
		dialog.Info("Kryptic", "The installer is open. Finish it to complete the update. Your sign-in is kept.")
		return
	}
	dialog.Info("Kryptic", "Updated to Kryptic "+result.Latest+". Your sign-in was kept.")
	reexecIfLinux()
}

func changeServerURL(client *api.Client, owned *server.Server) {
	current := authstore.ResolvedAPI()
	next, ok := dialog.PromptServerURI(current)
	if !ok {
		return
	}
	if err := login.SetActiveAPI(next); err != nil {
		dialog.Info("Kryptic", err.Error())
		return
	}
	client.BaseURL = next
	if owned != nil {
		owned.SetBaseURL(next)
	}
}
