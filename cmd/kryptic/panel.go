package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/dev-kryptic/daemon/internal/about"
	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/applog"
	"github.com/dev-kryptic/daemon/internal/authstore"
	"github.com/dev-kryptic/daemon/internal/config"
	"github.com/dev-kryptic/daemon/internal/dialog"
	"github.com/dev-kryptic/daemon/internal/ipc"
	"github.com/dev-kryptic/daemon/internal/login"
	"github.com/dev-kryptic/daemon/internal/manageui"
	"github.com/dev-kryptic/daemon/internal/server"
)

func runPanel() error {
	if runtime.GOOS == "darwin" {
		fmt.Println("Use Open Kryptic in the menu bar to manage this install.")
		return nil
	}

	client := api.NewClient()
	manageui.Show(manageui.Handlers{
		Snapshot: panelSnapshot,
		Action: func(name, arg string) {
			handlePanelAction(client, name, arg)
		},
	})
	manageui.Wait()
	return nil
}

func panelSnapshot() manageui.Snapshot {
	apiURL, _ := config.API()
	snap := manageui.Snapshot{
		Connection:      "signed_out",
		ConnectionLabel: manageui.ConnectionLabel("signed_out", ""),
		API:             apiURL,
		CanLogin:        true,
		UpdateTitle:     "Check for Updates",
		Version:         server.Version,
	}
	response, err := ipc.Request(map[string]any{"type": "status"})
	if err != nil {
		snap.Connection = "connecting"
		snap.ConnectionLabel = manageui.ConnectionLabel("connecting", "")
		return snap
	}
	snap.Running = true
	if reported, ok := response["apiUrl"].(string); ok && reported != "" {
		snap.API = reported
	}
	kind, _ := response["connection"].(string)
	if kind == "" {
		if response["authenticated"] == true {
			kind = "connected"
		} else {
			kind = "signed_out"
		}
	}
	email, _ := response["email"].(string)
	org, _ := response["organization"].(string)
	snap.Connection = kind
	snap.ConnectionLabel = manageui.ConnectionLabel(kind, email)
	snap.Email = email
	snap.Organization = org
	snap.Authenticated = response["authenticated"] == true
	snap.Profiles = manageui.ProfilesFromStatus(response["profiles"])
	return snap
}

func handlePanelAction(client *api.Client, name, arg string) {
	switch name {
	case "signIn":
		_, _ = login.Run(client, nil)
	case "addAccount":
		def, _ := config.API()
		value, ok := dialog.PromptNewAccountServer(def)
		if !ok {
			return
		}
		_, _ = login.RunAdd(api.NewClientFor(value), nil)
	case "signOut":
		_ = login.Logout(client)
	case "switchProfile":
		_ = login.Switch(arg)
	case "deleteProfile":
		_ = login.Delete(arg)
	case "serverURI":
		next := strings.TrimSpace(arg)
		if next == "" {
			var ok bool
			next, ok = dialog.PromptServerURI(authstore.ResolvedAPI())
			if !ok {
				return
			}
		}
		_ = login.SetActiveAPI(next)
		client.BaseURL = next
	case "flush":
		_, _ = ipc.Request(map[string]any{"type": "flush"})
	case "github":
		about.OpenGitHub()
	case "docs":
		about.OpenDocs()
	case "logs":
		_ = applog.Reveal()
	case "about":
		about.Show()
	case "quit":
		os.Exit(0)
	}
}
