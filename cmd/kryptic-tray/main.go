// The Kryptic tray app for Windows and Linux - the counterpart of the SwiftUI
// menu-bar app in daemon/macos. Unlike macOS (which supervises a bundled CLI as a
// child process), this runs the daemon in-process: one binary, one process.
// An externally managed daemon (systemd, `kryptic start`) is detected and left
// alone; the tray then acts as a remote control for it.
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"fyne.io/systray"
	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/ipc"
	"github.com/dev-kryptic/daemon/internal/login"
	"github.com/dev-kryptic/daemon/internal/server"
)

func main() {
	systray.Run(onReady, nil)
}

func onReady() {
	systray.SetIcon(trayIcon)
	systray.SetTooltip("Kryptic")

	statusItem := systray.AddMenuItem("Daemon: starting…", "")
	statusItem.Disable()
	orgItem := systray.AddMenuItem("", "")
	orgItem.Disable()
	orgItem.Hide()
	apiItem := systray.AddMenuItem("", "")
	apiItem.Disable()
	apiItem.Hide()
	if override := os.Getenv("KRYPTIC_API"); override != "" {
		apiItem.SetTitle("API: " + override)
		apiItem.Show()
	}

	systray.AddSeparator()
	codeItem := systray.AddMenuItem("", "")
	codeItem.Disable()
	codeItem.Hide()
	signInItem := systray.AddMenuItem("Sign In…", "Sign in via your browser")
	signOutItem := systray.AddMenuItem("Sign Out…", "Revoke this device's session")
	signOutItem.Hide()
	flushItem := systray.AddMenuItem("Refresh Secrets Cache", "Refetch secrets on the next request")
	systray.AddSeparator()
	quitItem := systray.AddMenuItem("Quit Kryptic", "")

	client := api.NewClient()

	// Serve in-process unless a daemon already answers on the socket/pipe.
	var ownedServer *server.Server
	if _, err := ipc.Request(map[string]any{"type": "status"}); err != nil {
		ownedServer = server.New(client)
		go func() {
			if err := ownedServer.Run(); err != nil {
				log.Printf("kryptic daemon exited: %v", err)
			}
		}()
	}

	refresh := func() {
		response, err := ipc.Request(map[string]any{"type": "status"})
		switch {
		case err != nil:
			statusItem.SetTitle("Daemon: offline")
			orgItem.Hide()
			signOutItem.Hide()
			signInItem.Show()
		case response["authenticated"] == true:
			statusItem.SetTitle(fmt.Sprintf("Daemon: online - %v", response["email"]))
			orgItem.SetTitle(fmt.Sprintf("%v", response["organization"]))
			orgItem.Show()
			signInItem.Hide()
			signOutItem.Show()
		default:
			statusItem.SetTitle("Daemon: online - not signed in")
			orgItem.Hide()
			signOutItem.Hide()
			signInItem.Show()
		}
	}

	go func() {
		time.Sleep(500 * time.Millisecond) // give the in-process listener a beat
		refresh()

		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				refresh()

			case <-signInItem.ClickedCh:
				signInItem.Disable()
				go func() {
					_, err := login.Run(client, func(userCode, _ string) {
						codeItem.SetTitle("Confirm code in browser: " + userCode)
						codeItem.Show()
					})
					if err != nil {
						codeItem.SetTitle("⚠️ " + err.Error())
						codeItem.Show()
					} else {
						codeItem.Hide()
					}
					signInItem.Enable()
					refresh()
				}()

			case <-signOutItem.ClickedCh:
				go func() {
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

			case <-quitItem.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}
