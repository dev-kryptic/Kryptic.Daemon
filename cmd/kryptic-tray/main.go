// The Kryptic tray app for Windows and Linux - the counterpart of the SwiftUI
// menu-bar app in macos/. Unlike macOS (which supervises a bundled CLI as a
// child process), this runs the daemon in-process: one binary, one process.
// An externally managed daemon (systemd, `kryptic start`) is detected and left
// alone; the tray then acts as a remote control for it.
//
// The menu is a 1:1 match of the macOS MenuBarExtra: status, sign in/out,
// cancel sign-in, refresh cache, About, quit.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/systray"
	"github.com/dev-kryptic/daemon/internal/about"
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

	apiItem := systray.AddMenuItem("", "")
	apiItem.Disable()
	apiItem.Hide()
	if override := os.Getenv("KRYPTIC_API"); override != "" {
		apiItem.SetTitle("API: " + override)
		apiItem.Show()
	}

	statusItem := systray.AddMenuItem("Daemon: starting…", "")
	statusItem.Disable()
	orgItem := systray.AddMenuItem("", "")
	orgItem.Disable()
	orgItem.Hide()

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
	aboutItem := systray.AddMenuItem("About Kryptic…", "About Kryptic")
	systray.AddSeparator()
	quitItem := systray.AddMenuItem("Quit Kryptic", "")

	client := api.NewClient()

	var ownedServer *server.Server
	if _, err := ipc.Request(map[string]any{"type": "status"}); err != nil {
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
		switch {
		case response["authenticated"] == true:
			statusItem.SetTitle(fmt.Sprintf("Daemon: online - %v", response["email"]))
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

			case <-aboutItem.ClickedCh:
				about.Show()

			case <-quitItem.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}
