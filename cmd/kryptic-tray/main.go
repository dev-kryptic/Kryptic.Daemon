// The Kryptic tray app for Windows and Linux - the counterpart of the SwiftUI
// menu-bar app in macos/. Unlike macOS (which supervises a bundled CLI as a
// child process), this runs the daemon in-process: one binary, one process.
// An externally managed daemon (systemd, `kryptic start`) is detected and left
// alone; the tray then acts as a remote control for it.
//
// The tray menu is native. Open Kryptic shows the native window.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/systray"
	"github.com/dev-kryptic/daemon/internal/about"
	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/applog"
	"github.com/dev-kryptic/daemon/internal/authstore"
	"github.com/dev-kryptic/daemon/internal/config"
	"github.com/dev-kryptic/daemon/internal/dialog"
	"github.com/dev-kryptic/daemon/internal/ipc"
	"github.com/dev-kryptic/daemon/internal/login"
	"github.com/dev-kryptic/daemon/internal/manageui"
	"github.com/dev-kryptic/daemon/internal/pidfile"
	"github.com/dev-kryptic/daemon/internal/server"
	"github.com/dev-kryptic/daemon/internal/singleinstance"
	"github.com/dev-kryptic/daemon/internal/update"
)

// trayLock is held for the process lifetime. The re-exec fallback releases it
// before spawning the replacement, so the new instance can take it.
var trayLock func()

func releaseTrayLock() {
	if trayLock != nil {
		trayLock()
		trayLock = nil
	}
}

func main() {
	release, ok := singleinstance.Acquire("kryptic-tray")
	if !ok {
		// A second launch (autostart plus a manual click, say) is a no-op:
		// one tray icon per user, never two.
		log.Println("kryptic-tray is already running for this user")
		return
	}
	trayLock = release
	systray.Run(onReady, nil)
}

func onReady() {
	setTrayConnection(trayConnecting)
	systray.SetIcon(currentTrayIcon())
	systray.SetTooltip(trayTooltip(trayConnecting))
	go watchTrayTheme()
	go ensureLauncherIcon()
	go ensureAutostart()

	statusItem := systray.AddMenuItem("Signed out", "")
	statusItem.Disable()
	apiItem := systray.AddMenuItem("", "")
	apiItem.Disable()
	systray.AddSeparator()

	signInItem := systray.AddMenuItem("Sign In…", "Sign in via your browser")
	signOutItem := systray.AddMenuItem("Sign Out…", "Sign out of the active account")
	cancelItem := systray.AddMenuItem("Cancel Sign-In", "")
	signOutItem.Hide()
	cancelItem.Hide()

	accountsMenu := systray.AddMenuItem("Accounts", "")
	const maxProfileItems = 8
	profileItems := make([]*systray.MenuItem, maxProfileItems)
	profileIDs := make([]string, maxProfileItems)
	for i := range profileItems {
		profileItems[i] = accountsMenu.AddSubMenuItem("Account", "")
		profileItems[i].Hide()
	}
	addAccountItem := accountsMenu.AddSubMenuItem("Add Account…", "")
	deleteItems := make([]*systray.MenuItem, maxProfileItems)
	for i := range deleteItems {
		deleteItems[i] = accountsMenu.AddSubMenuItem("Remove Account", "")
		deleteItems[i].Hide()
	}

	openItem := systray.AddMenuItem("Open Kryptic", "Manage accounts, cache, and settings")
	systray.AddSeparator()

	opsMenu := systray.AddMenuItem("Operations", "")
	flushItem := opsMenu.AddSubMenuItem("Refresh Secrets Cache", "")
	scanItem := opsMenu.AddSubMenuItem("Scan for secrets", "")

	settingsMenu := systray.AddMenuItem("Settings", "")
	updateItem := settingsMenu.AddSubMenuItem("Check for Updates", "")
	serverItem := settingsMenu.AddSubMenuItem("Server URI", "")

	helpMenu := systray.AddMenuItem("Help & Support", "")
	githubItem := helpMenu.AddSubMenuItem("GitHub", "")
	docsItem := helpMenu.AddSubMenuItem("Documentation", "")
	logsItem := helpMenu.AddSubMenuItem("Reveal Diagnostics Log", "")

	aboutItem := systray.AddMenuItem("About Kryptic", "")
	systray.AddSeparator()
	quitItem := systray.AddMenuItem("Quit Kryptic", "")

	client := api.NewClient()

	var ownedServer *server.Server
	var wrotePidfile bool
	if shouldStartInProcess() {
		if err := pidfile.Write(); err == nil {
			wrotePidfile = true
		}
		ownedServer = server.New(client)
		go func() {
			if err := ownedServer.Run(); err != nil {
				log.Printf("kryptic daemon exited: %v", err)
				applog.Error("tray", "daemon.exit", err)
			}
		}()
	}

	var loginInProgress atomic.Bool
	var loginMu sync.Mutex
	var loginCancel context.CancelFunc
	var lastIconKind string
	panel := &panelState{}
	panel.setUpdateTitle("Check for Updates")

	applyIcon := func(kind string) {
		if kind == lastIconKind {
			return
		}
		lastIconKind = kind
		setTrayConnection(kind)
		systray.SetIcon(currentTrayIcon())
		systray.SetTooltip(trayTooltip(kind))
	}

	var startLogin func(add bool)

	refresh := func() {
		response, err := ipc.Request(map[string]any{"type": "status"})
		if err != nil {
			kind := trayConnecting
			applyIcon(kind)
			panel.setStatus(kind, "", "", "", false, false, nil)
			if !loginInProgress.Load() {
				signOutItem.Hide()
				cancelItem.Hide()
				signInItem.Show()
			}
			statusItem.SetTitle("Connecting…")
			return
		}

		apiURL, _ := response["apiUrl"].(string)
		if apiURL == "" {
			apiURL = authstore.ResolvedAPI()
		}
		client.BaseURL = apiURL
		kind, _ := response["connection"].(string)
		if loginInProgress.Load() {
			kind = trayConnecting
		}
		if kind == "" {
			if response["authenticated"] == true {
				kind = trayConnected
			} else {
				kind = traySignedOut
			}
		}
		email, _ := response["email"].(string)
		org, _ := response["organization"].(string)
		applyIcon(kind)
		profiles := manageui.ProfilesFromStatus(response["profiles"])
		panel.setStatus(kind, email, org, apiURL, response["authenticated"] == true, true, profiles)

		label := manageui.ConnectionLabel(kind, email)
		if email != "" {
			label += " · " + email
		}
		statusItem.SetTitle(label)
		if apiURL != "" {
			apiItem.SetTitle(manageui.HostLabel(apiURL))
			apiItem.Show()
		} else {
			apiItem.Hide()
		}

		shown := 0
		for _, profile := range profiles {
			if shown >= maxProfileItems {
				break
			}
			profileItems[shown].SetTitle(profile.Title())
			profileItems[shown].Show()
			if profile.Active {
				profileItems[shown].Disable()
			} else {
				profileItems[shown].Enable()
			}
			profileIDs[shown] = profile.ID
			name := profile.Email
			if name == "" {
				name = profile.ID
			}
			deleteItems[shown].SetTitle("Remove " + name)
			deleteItems[shown].Show()
			shown++
		}
		for i := shown; i < maxProfileItems; i++ {
			profileItems[i].Hide()
			deleteItems[i].Hide()
			profileIDs[i] = ""
		}

		if loginInProgress.Load() {
			signInItem.Hide()
			signOutItem.Hide()
			cancelItem.Show()
		} else if response["authenticated"] == true {
			signInItem.Hide()
			cancelItem.Hide()
			signOutItem.Show()
		} else {
			signOutItem.Hide()
			cancelItem.Hide()
			signInItem.Show()
		}
	}

	startLogin = func(add bool) {
		loginClient := client
		if add {
			if config.EnvOverrides() {
				dialog.Info("Kryptic", "KRYPTIC_API is set and overrides the server URI for new accounts.")
			} else {
				def, _ := config.API()
				value, ok := dialog.Prompt("Kryptic", "Server URI for the new account", def)
				if !ok {
					return
				}
				value = strings.TrimSpace(value)
				if value == "" {
					value = config.DefaultAPI
				}
				normalized, err := config.NormalizeAPI(value)
				if err != nil {
					dialog.Info("Kryptic", err.Error())
					return
				}
				loginClient = api.NewClientFor(normalized)
			}
		} else {
			loginClient = api.NewClientFor(authstore.ResolvedAPI())
		}

		loginMu.Lock()
		if loginCancel != nil {
			loginMu.Unlock()
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		loginCancel = cancel
		loginMu.Unlock()

		loginInProgress.Store(true)
		signInItem.Hide()
		signOutItem.Hide()
		cancelItem.Show()
		applyIcon(trayConnecting)
		panel.setLogin(true, "", "")

		go func() {
			defer func() {
				loginMu.Lock()
				loginCancel = nil
				loginMu.Unlock()
				loginInProgress.Store(false)
				panel.setLogin(false, "", "")
				refresh()
			}()

			_, err := login.RunContext(ctx, loginClient, func(userCode, _ string) {
				panel.setLogin(true, userCode, "")
			}, add)
			switch {
			case err == nil, errors.Is(err, context.Canceled):
				panel.setLogin(false, "", "")
			default:
				panel.setLogin(false, "", err.Error())
			}
		}()
	}

	doSignOut := func() {
		if !dialog.Confirm("Kryptic",
			"Signing out ends this account's session and drops secrets from memory. "+
				"Other saved accounts stay signed in. This machine's keys stay, so the next "+
				"sign-in does not need a new admin grant unless durable device trust is off.\n\nSign out?") {
			return
		}
		_ = login.Logout(client)
		if ownedServer != nil {
			ownedServer.ResetAuth()
		}
		refresh()
	}

	doQuit := func() {
		if wrotePidfile {
			pidfile.Remove()
		}
		systray.Quit()
	}

	doSwitch := func(id string) {
		if id == "" {
			return
		}
		_, _ = ipc.Request(map[string]any{"type": "switch-profile", "profileId": id})
		if ownedServer != nil {
			ownedServer.ResetAuth()
			ownedServer.SyncActiveAPI()
		}
		refresh()
	}

	doDelete := func(id string) {
		if id == "" {
			return
		}
		if !dialog.Confirm("Kryptic",
			"This removes that account from this install and revokes its device on that server. Other profiles are not touched.") {
			return
		}
		_, _ = ipc.Request(map[string]any{"type": "delete-profile", "profileId": id})
		if ownedServer != nil {
			ownedServer.ResetAuth()
			ownedServer.SyncActiveAPI()
		}
		refresh()
	}

	handlers := manageui.Handlers{
		Snapshot: panel.snapshot,
		Action: func(name, arg string) {
			switch name {
			case "signIn":
				startLogin(false)
			case "addAccount":
				startLogin(true)
			case "cancelLogin":
				loginMu.Lock()
				if loginCancel != nil {
					loginCancel()
				}
				loginMu.Unlock()
			case "signOut":
				doSignOut()
			case "switchProfile":
				doSwitch(arg)
			case "deleteProfile":
				doDelete(arg)
			case "flush":
				_, _ = ipc.Request(map[string]any{"type": "flush"})
			case "scan":
				panel.setScan(true)
				runFolderScan(scanItem)
				panel.setScan(false)
			case "update":
				panel.setUpdateTitle("Checking for Updates…")
				runUpdateFlow(updateItem)
				panel.setUpdateTitle("Check for Updates")
			case "serverURI":
				changeServerURL(client, ownedServer)
				refresh()
			case "github":
				about.OpenGitHub()
			case "docs":
				about.OpenDocs()
			case "logs":
				if err := applog.Reveal(); err != nil {
					dialog.Info("Kryptic", "Could not open the diagnostics log.")
				}
			case "about":
				about.Show()
			case "quit":
				doQuit()
			}
		},
	}
	openWindow := func() {
		manageui.Show(handlers)
	}
	systray.SetOnTapped(openWindow)

	listen := func(item *systray.MenuItem, fn func()) {
		go func() {
			for range item.ClickedCh {
				fn()
			}
		}()
	}
	listen(openItem, openWindow)
	listen(signInItem, func() { startLogin(false) })
	listen(signOutItem, func() { go doSignOut() })
	listen(cancelItem, func() {
		loginMu.Lock()
		if loginCancel != nil {
			loginCancel()
		}
		loginMu.Unlock()
	})
	listen(addAccountItem, func() { startLogin(true) })
	listen(flushItem, func() { _, _ = ipc.Request(map[string]any{"type": "flush"}) })
	listen(scanItem, func() { runFolderScan(scanItem) })
	listen(updateItem, func() { runUpdateFlow(updateItem) })
	listen(serverItem, func() {
		changeServerURL(client, ownedServer)
		refresh()
	})
	listen(githubItem, about.OpenGitHub)
	listen(docsItem, about.OpenDocs)
	listen(logsItem, func() {
		if err := applog.Reveal(); err != nil {
			dialog.Info("Kryptic", "Could not open the diagnostics log.")
		}
	})
	listen(aboutItem, about.Show)
	listen(quitItem, doQuit)
	for i := range profileItems {
		i := i
		listen(profileItems[i], func() { doSwitch(profileIDs[i]) })
		listen(deleteItems[i], func() { doDelete(profileIDs[i]) })
	}

	go func() {
		time.Sleep(500 * time.Millisecond)
		refresh()
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			refresh()
		}
	}()

	go watchForUpdates(panel, updateItem)
}

// shouldStartInProcess is true when nothing is serving the socket, or when the
// process that is serving it is an older install. Stopping that process does
// not clear the login session.
func shouldStartInProcess() bool {
	response, err := ipc.Request(map[string]any{"type": "status"})
	if err != nil {
		return true
	}
	running, _ := response["daemonVersion"].(string)
	if running == server.Version {
		return false
	}
	log.Printf("replacing daemon %s with %s (session kept)", running, server.Version)
	applog.Event("tray", "daemon.replace", "session=kept")
	if err := pidfile.StopRunning(); err != nil {
		log.Printf("could not stop previous daemon: %v", err)
		return false
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := ipc.Request(map[string]any{"type": "status"}); err != nil {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return true
}

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
	if config.EnvOverrides() {
		dialog.Info("Kryptic", "KRYPTIC_API is set in the environment and overrides every profile's URL.")
		return
	}
	current := authstore.ResolvedAPI()
	value, ok := dialog.Prompt("Kryptic", "Server URI for this profile", current)
	if !ok {
		return
	}
	value = strings.TrimSpace(value)
	var next string
	if value == "" {
		next = config.DefaultAPI
	} else {
		normalized, err := config.NormalizeAPI(value)
		if err != nil {
			dialog.Info("Kryptic", err.Error())
			return
		}
		next = normalized
	}
	if next == current {
		return
	}
	if !dialog.Confirm("Kryptic", "This signs this profile out of the previous server. Other profiles keep their URL and session.") {
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
