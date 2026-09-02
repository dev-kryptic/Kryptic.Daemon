// The Kryptic tray app for Windows and Linux - the counterpart of the SwiftUI
// menu-bar app in macos/. Unlike macOS (which supervises a bundled CLI as a
// child process), this runs the daemon in-process: one binary, one process.
// An externally managed daemon (systemd, `kryptic start`) is detected and left
// alone; the tray then acts as a remote control for it.
//
// The menu is a 1:1 match of the macOS MenuBarExtra: status, Scan…, sign in/out,
// cancel sign-in, refresh cache, Check for Updates, Server URL, About, quit.
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
	"github.com/dev-kryptic/daemon/internal/config"
	"github.com/dev-kryptic/daemon/internal/dialog"
	"github.com/dev-kryptic/daemon/internal/ipc"
	"github.com/dev-kryptic/daemon/internal/login"
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
	systray.SetIcon(currentTrayIcon())
	systray.SetTooltip("Kryptic")
	go watchTrayTheme()
	go ensureLauncherIcon()
	go ensureAutostart()

	apiItem := systray.AddMenuItem("", "")
	apiItem.Disable()
	showAPI(apiItem, "")

	statusItem := systray.AddMenuItem("Daemon: starting…", "")
	statusItem.Disable()
	orgItem := systray.AddMenuItem("", "")
	orgItem.Disable()
	orgItem.Hide()

	systray.AddSeparator()
	scanItem := systray.AddMenuItem("Scan…", "Scan a folder for leaked secrets (offline, no sign-in)")
	systray.AddSeparator()
	codeItem := systray.AddMenuItem("", "")
	codeItem.Disable()
	codeItem.Hide()
	cancelItem := systray.AddMenuItem("Cancel Sign-In", "Stop the browser sign-in")
	cancelItem.Hide()
	signInItem := systray.AddMenuItem("Sign In…", "Sign in via your browser")
	signOutItem := systray.AddMenuItem("Sign Out…", "Revoke this device's session")
	signOutItem.Hide()
	flushItem := systray.AddMenuItem("Refresh Secrets Cache", "Refetch secrets on the next request")
	flushItem.Disable()
	updateItem := systray.AddMenuItem("Check for Updates…", "Install the latest Kryptic release")
	serverItem := systray.AddMenuItem("Server URL…", "Point the daemon at a different Kryptic server")
	aboutItem := systray.AddMenuItem("About Kryptic…", "About Kryptic")
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
			}
		}()
	}

	var loginInProgress atomic.Bool
	var loginMu sync.Mutex
	var loginCancel context.CancelFunc

	refresh := func() {
		response, err := ipc.Request(map[string]any{"type": "status"})
		if err != nil {
			statusItem.SetTitle("Daemon: starting…")
			orgItem.Hide()
			flushItem.Disable()
			if !loginInProgress.Load() {
				signOutItem.Hide()
				signInItem.Show()
			}
			return
		}

		flushItem.Enable()
		if apiURL, ok := response["apiUrl"].(string); ok {
			showAPI(apiItem, apiURL)
		}
		switch {
		case response["authenticated"] == true:
			granted, hasGrantField := response["orgKeyGranted"].(bool)
			if hasGrantField && !granted {
				statusItem.SetTitle("Daemon: online - waiting for organization key")
			} else {
				statusItem.SetTitle(fmt.Sprintf("Daemon: online - %v", response["email"]))
			}
			if org, ok := response["organization"].(string); ok && org != "" {
				orgItem.SetTitle(org)
				orgItem.Show()
			} else {
				orgItem.Hide()
			}
			if !loginInProgress.Load() {
				signInItem.Hide()
				signOutItem.Show()
			}
		default:
			statusItem.SetTitle("Daemon: online - not signed in")
			orgItem.Hide()
			if !loginInProgress.Load() {
				signOutItem.Hide()
				signInItem.Show()
			}
		}
	}

	go func() {
		time.Sleep(500 * time.Millisecond)
		refresh()

		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				refresh()

			case <-scanItem.ClickedCh:
				go runFolderScan(scanItem)

			case <-signInItem.ClickedCh:
				loginMu.Lock()
				if loginCancel != nil {
					loginMu.Unlock()
					continue
				}
				ctx, cancel := context.WithCancel(context.Background())
				loginCancel = cancel
				loginMu.Unlock()

				loginInProgress.Store(true)
				signInItem.Hide()
				signOutItem.Hide()
				cancelItem.Show()
				codeItem.Hide()

				go func() {
					defer func() {
						loginMu.Lock()
						loginCancel = nil
						loginMu.Unlock()
						loginInProgress.Store(false)
						cancelItem.Hide()
						refresh()
					}()

					_, err := login.RunContext(ctx, client, func(userCode, _ string) {
						codeItem.SetTitle("Confirm code in browser: " + userCode)
						codeItem.Show()
					})
					switch {
					case err == nil, errors.Is(err, context.Canceled):
						codeItem.Hide()
					default:
						codeItem.SetTitle("⚠️ " + err.Error())
						codeItem.Show()
					}
				}()

			case <-cancelItem.ClickedCh:
				loginMu.Lock()
				if loginCancel != nil {
					loginCancel()
				}
				loginMu.Unlock()

			case <-signOutItem.ClickedCh:
				go func() {
					// Signing out destroys the device key, so the org-key
					// grant is lost for good - never do it silently.
					if !dialog.Confirm("Kryptic",
						"Signing out deletes this device's encryption key. "+
							"When you sign in again, an admin must grant the organization key "+
							"to this device again before it can decrypt any secrets.\n\nSign out?") {
						return
					}
					_ = login.Logout(client)
					if ownedServer != nil {
						ownedServer.ResetAuth()
					}
					refresh()
				}()

			case <-flushItem.ClickedCh:
				go func() {
					_, _ = ipc.Request(map[string]any{"type": "flush"})
				}()

			case <-updateItem.ClickedCh:
				go runUpdateFlow(updateItem)

			case <-serverItem.ClickedCh:
				go func() {
					changeServerURL(client, ownedServer)
					showAPI(apiItem, client.BaseURL)
					refresh()
				}()

			case <-aboutItem.ClickedCh:
				about.Show()

			case <-quitItem.ClickedCh:
				if wrotePidfile {
					pidfile.Remove()
				}
				systray.Quit()
				return
			}
		}
	}()

	go watchForUpdates(updateItem)
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

func showAPI(item *systray.MenuItem, reported string) {
	url := reported
	if url == "" {
		url, _ = config.API()
	}
	item.SetTitle("API: " + url)
	item.Show()
}

func watchForUpdates(item *systray.MenuItem) {
	time.Sleep(8 * time.Second)
	for {
		result, err := update.Check(server.Version)
		if err == nil && result.Newer {
			item.SetTitle("Update Available…")
		}
		time.Sleep(12 * time.Hour)
	}
}

func runUpdateFlow(item *systray.MenuItem) {
	item.SetTitle("Checking for Updates…")
	result, err := update.Check(server.Version)
	item.SetTitle("Check for Updates…")
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
		item.SetTitle("Update Available…")
		return
	}
	item.SetTitle("Updating…")
	var progress dialog.Progress
	if !update.PreferInstaller() {
		progress = dialog.OpenProgress("Kryptic", "Updating…")
		defer progress.Close()
	}
	err = update.ApplyWithProgress(server.Version, func(percent int, message string) {
		item.SetTitle(fmt.Sprintf("Updating… %d%%", percent))
		if progress != nil {
			progress.Set(percent, message)
		}
	})
	if progress != nil {
		progress.Close()
	}
	item.SetTitle("Check for Updates…")
	if err != nil {
		dialog.Info("Kryptic", "Update failed: "+err.Error())
		item.SetTitle("Update Available…")
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
		dialog.Info("Kryptic", "KRYPTIC_API is set in the environment and overrides the saved URL.")
		return
	}
	current, _ := config.API()
	value, ok := dialog.Prompt("Kryptic", "Daemon server URL", current)
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
	if !dialog.Confirm("Kryptic", "Changing the server signs you out of the current one. Continue?") {
		return
	}
	_ = login.Logout(api.NewClientFor(current))
	var err error
	if next == config.DefaultAPI {
		err = config.ResetAPI()
	} else {
		err = config.SetAPI(next)
	}
	if err != nil {
		dialog.Info("Kryptic", err.Error())
		return
	}
	next, _ = config.API()
	client.BaseURL = next
	if owned != nil {
		owned.SetBaseURL(next)
		return
	}
	update.RestartDaemon()
}
