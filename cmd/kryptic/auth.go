package main

import (
	"fmt"
	"os"

	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/authstore"
	"github.com/dev-kryptic/daemon/internal/config"
	"github.com/dev-kryptic/daemon/internal/login"
)

func runLogin(client *api.Client) error {
	add := hasArg("--add")
	target := client.BaseURL
	if raw, ok := argValue("--api"); ok {
		normalized, err := config.NormalizeAPI(raw)
		if err != nil {
			return err
		}
		target = normalized
	} else if add {
		target, _ = config.API()
	} else {
		target = authstore.ResolvedAPI()
	}
	client = api.NewClientFor(target)
	run := login.Run
	if add {
		run = login.RunAdd
	}
	me, err := run(client, func(userCode, verificationURL string) {
		fmt.Printf("Confirm this code in your browser: %s\n%s\n", userCode, verificationURL)
	})
	if err != nil {
		return err
	}
	fmt.Printf("Signed in as %s (%s). The daemon can now serve secrets.\n", me.Email, me.Organization)
	return nil
}

func runProfile() error {
	args := os.Args[2:]
	if len(args) == 0 || args[0] == "list" {
		return listProfiles()
	}
	if args[0] == "switch" {
		if len(args) < 2 {
			return fmt.Errorf("usage: kryptic profile switch <id|email>")
		}
		if err := login.Switch(args[1]); err != nil {
			return err
		}
		fmt.Printf("Switched to %s. Other accounts stay signed in.\n", args[1])
		return nil
	}
	if args[0] == "delete" || args[0] == "remove" {
		if len(args) < 2 {
			return fmt.Errorf("usage: kryptic profile delete <id|email>")
		}
		if !confirmed("Delete this profile? [y/N] ",
			"This removes that account from this install and revokes its device on that server.",
			"Other profiles are not touched.") {
			fmt.Println("Delete cancelled.")
			return nil
		}
		if err := login.Delete(args[1]); err != nil {
			return err
		}
		fmt.Printf("Deleted %s from this install.\n", args[1])
		return nil
	}
	return fmt.Errorf("usage: kryptic profile [list|switch <id|email>|delete <id|email>]")
}

func listProfiles() error {
	store, err := authstore.LoadStore()
	if err != nil {
		fmt.Println("No saved accounts. Run `kryptic login`.")
		return nil
	}
	for _, p := range store.Profiles {
		mark := " "
		if p.ID == store.ActiveID {
			mark = "*"
		}
		state := "signed out"
		if p.SignedIn() {
			state = "signed in"
		}
		label := p.Email
		if label == "" {
			label = p.ID
		}
		if p.Organization != "" {
			label += " @ " + p.Organization
		}
		fmt.Printf("%s %s  (%s, %s, %s)\n", mark, label, state, p.API(), p.ID)
	}
	return nil
}

func runLogout(client *api.Client) error {
	if !confirmed("Sign out anyway? [y/N] ",
		"Signing out revokes this session and drops secrets from memory.",
		"This machine's encryption keys stay, so the next sign-in does not",
		"need a new admin grant (unless durable device trust is off).") {
		fmt.Println("Logout cancelled - you are still signed in.")
		return nil
	}
	if err := login.Logout(client); err != nil {
		return err
	}
	fmt.Println("Signed out. This machine's encryption keys were kept, so the next")
	fmt.Println("`kryptic login` does not need a new admin grant unless durable device")
	fmt.Println("trust is off. Use `kryptic reset-device` to wipe the keys.")
	return nil
}

func runResetDevice(client *api.Client) error {
	all := hasArg("--all")
	details := []string{"This deletes the active profile's encryption keys on this install."}
	if all {
		details = []string{"This deletes every profile's encryption keys on this install."}
	}
	details = append(details, "The next `kryptic login` needs a new admin grant under Approvals.")
	if !confirmed("Reset this device? [y/N] ", details...) {
		fmt.Println("Reset cancelled - this machine's keys are still here.")
		return nil
	}
	if all {
		if err := login.ResetAllDevices(client); err != nil {
			return err
		}
		fmt.Println("Every profile on this install was wiped. After the next")
		fmt.Println("`kryptic login`, an admin must grant the organization key under Approvals.")
		return nil
	}
	if err := login.ResetDevice(client); err != nil {
		return err
	}
	fmt.Println("This profile's keys were deleted. After your next `kryptic login`,")
	fmt.Println("an admin must grant the organization key to this machine under Approvals.")
	return nil
}

func whoami(client *api.Client) error {
	accessToken, err := login.AccessToken(client)
	if err != nil {
		return err
	}
	me, err := client.Me(accessToken)
	if err != nil {
		return err
	}
	fmt.Printf("%s (%s) - organization: %s\n", me.Email, me.DisplayName, me.Organization)
	if me.HasOrgKeyGrant != nil && !*me.HasOrgKeyGrant {
		fmt.Println("organization key: not granted (an admin must approve this device under Approvals)")
	}
	return nil
}
