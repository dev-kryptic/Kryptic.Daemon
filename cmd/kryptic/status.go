package main

import (
	"fmt"

	"github.com/dev-kryptic/daemon/internal/config"
	"github.com/dev-kryptic/daemon/internal/ipc"
)

func status() error {
	apiURL, source := config.API()
	response, err := ipc.Request(map[string]any{"type": "status"})
	if err != nil {
		fmt.Println("daemon: not running")
		fmt.Printf("api: %s (%s)\n", apiURL, source)
		return nil
	}
	if response["authenticated"] == true {
		email, _ := response["email"].(string)
		org, _ := response["organization"].(string)
		if email == "" {
			fmt.Printf("daemon: online (v%v) - signed in\n", response["daemonVersion"])
		} else {
			fmt.Printf("daemon: online (v%v) - signed in as %s @ %s\n",
				response["daemonVersion"], email, org)
		}
		if granted, ok := response["orgKeyGranted"].(bool); ok && !granted {
			fmt.Println("organization key: not granted (an admin must approve this device under Approvals)")
		}
	} else {
		fmt.Printf("daemon: online (v%v) - not signed in (run `kryptic login`)\n", response["daemonVersion"])
	}
	if connection, ok := response["connection"].(string); ok && connection != "" {
		fmt.Printf("connection: %s\n", connection)
	}
	if raw, ok := response["profiles"].([]any); ok && len(raw) > 1 {
		fmt.Printf("profiles: %d saved (kryptic profile)\n", len(raw))
	}
	if reported, ok := response["apiUrl"].(string); ok && reported != "" {
		apiURL = reported
	}
	fmt.Printf("api: %s (%s)\n", apiURL, source)
	return nil
}
