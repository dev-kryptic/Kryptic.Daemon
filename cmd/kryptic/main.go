// The kryptic CLI. `kryptic start` runs the daemon in the foreground
// (launchd/systemd/the service manager keep it alive); the other commands talk
// to the platform or to the running daemon's socket.
//
//go:generate go run github.com/tc-hib/go-winres@v0.3.3 make --arch amd64 --in winres/winres.json --out rsrc
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/server"
	"github.com/dev-kryptic/daemon/internal/update"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}

	client := api.NewClient()

	var err error
	switch os.Args[1] {
	case "start":
		err = runStart(client)
	case "stop":
		err = runStop()
	case "update":
		err = runUpdate()
	case "config":
		err = runConfig()
	case "scan":
		err = runScan()
	case "login":
		err = runLogin(client)
	case "logout":
		err = runLogout(client)
	case "profile":
		err = runProfile()
	case "reset-device":
		err = runResetDevice(client)
	case "status":
		err = status()
	case "whoami":
		err = whoami(client)
	case "secrets":
		err = secrets()
	case "connector":
		err = connector()
	case "ci":
		err = ci()
	case "flush":
		err = flush()
	case "logs":
		err = runLogs()
	case "panel":
		err = runPanel()
	case "version":
		fmt.Println("kryptic", server.Version)
	default:
		usage()
	}

	if err != nil {
		if errors.Is(err, update.ErrAvailable) {
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, "kryptic:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`kryptic - the Kryptic daemon and CLI

  kryptic start                 run the daemon (foreground; managed by launchd/systemd)
  kryptic stop                  stop the running daemon
  kryptic login                 sign in via your browser (device flow)
  kryptic login --add           sign in another account without leaving the current one
  kryptic login --add --api URL sign in against a different Daemon BFF (self-host vs cloud)
  kryptic logout                revoke the active profile's session (asks to confirm; --yes skips)
  kryptic profile               list saved accounts on this install
  kryptic profile switch ID     switch the active account (email or profile id)
  kryptic profile delete ID     remove an account from this install (revokes that profile)
  kryptic reset-device          delete the active profile's keys (asks to confirm; --yes skips)
  kryptic reset-device --all    wipe every profile on this install
  kryptic status                daemon + session status
  kryptic whoami                the signed-in user and organization
  kryptic secrets list          projects and environments you can pull
  kryptic secrets get KEY --project proj_x --env development
  kryptic secrets export --project proj_x --env development   print a dotenv (decrypted locally)
  kryptic flush                 clear the daemon's secrets cache (refetch on next request)
  kryptic logs                  print the diagnostics log path (send this file to support)
  kryptic logs --reveal         open the diagnostics log in the file manager
  kryptic ci export --project proj_x --env production   pipeline secrets, decrypted locally
  kryptic ci revoke [--lease ID]        drop roles minted by this job
  kryptic connector run [--name NAME]   reap leftover dynamic-secret revokes
  kryptic scan [PATH]           scan files for leaked secrets (--staged, --export [FILE|DIR])
  kryptic update                update kryptic to the latest release
  kryptic update --check        report whether a newer release exists (exit 2 if so)
  kryptic update --installer    download the signed installer and open it (macOS/Windows)
  kryptic config                show the Daemon BFF URL
  kryptic config set-api URL    save the active profile's server URL (that profile only)
  kryptic config reset-api      return to https://daemon.kryptic.dev
  kryptic panel                 open Open Kryptic (Windows/Linux; on macOS use the menu)
  kryptic version`)
}
