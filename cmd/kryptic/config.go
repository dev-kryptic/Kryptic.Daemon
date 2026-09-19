package main

import (
	"fmt"
	"os"

	"github.com/dev-kryptic/daemon/internal/applog"
	"github.com/dev-kryptic/daemon/internal/authstore"
	"github.com/dev-kryptic/daemon/internal/config"
	"github.com/dev-kryptic/daemon/internal/login"
	"github.com/dev-kryptic/daemon/internal/server"
	"github.com/dev-kryptic/daemon/internal/update"
)

func runConfig() error {
	args := os.Args[2:]
	if len(args) == 0 {
		url, source := config.Resolve(activeProfileAPI())
		fmt.Printf("api: %s (%s)\n", url, source)
		if config.EnvOverrides() {
			fmt.Println("KRYPTIC_API is set and overrides every profile's URL.")
		}
		return nil
	}
	switch args[0] {
	case "set-api":
		if len(args) < 2 {
			return fmt.Errorf("usage: kryptic config set-api URL")
		}
		return applyProfileAPI(args[1])
	case "reset-api":
		return applyProfileAPI(config.DefaultAPI)
	default:
		return fmt.Errorf("usage: kryptic config [set-api URL|reset-api]")
	}
}

func activeProfileAPI() string {
	store, err := authstore.LoadStore()
	if err != nil {
		return ""
	}
	p, ok := store.Active()
	if !ok {
		return ""
	}
	return p.Config.API
}

func applyProfileAPI(raw string) error {
	previous := authstore.ResolvedAPI()
	if err := login.SetActiveAPI(raw); err != nil {
		return err
	}
	next, source := config.Resolve(activeProfileAPI())
	fmt.Printf("api: %s (%s)\n", next, source)
	if config.EnvOverrides() {
		fmt.Println("KRYPTIC_API is set and still overrides the saved URL.")
		return nil
	}
	if previous != next {
		fmt.Println("this profile was signed out of the previous server. Other profiles are unchanged.")
	}
	return nil
}

func runUpdate() error {
	checkOnly := false
	useInstaller := false
	for _, arg := range os.Args[2:] {
		switch arg {
		case "--check":
			checkOnly = true
		case "--installer":
			useInstaller = true
		default:
			return fmt.Errorf("usage: kryptic update [--check|--installer]")
		}
	}
	if checkOnly {
		return update.PrintCheck(server.Version)
	}
	if useInstaller {
		return update.RunInstaller(server.Version)
	}
	return update.Run(server.Version)
}

func runLogs() error {
	reveal := false
	for _, arg := range os.Args[2:] {
		if arg == "--reveal" || arg == "--open" {
			reveal = true
		}
	}
	text, err := applog.StatusLine()
	if err != nil {
		return err
	}
	fmt.Println(text)
	if reveal {
		return applog.Reveal()
	}
	return nil
}
