package main

import (
	"log"
	"time"

	"github.com/dev-kryptic/daemon/internal/applog"
	"github.com/dev-kryptic/daemon/internal/ipc"
	"github.com/dev-kryptic/daemon/internal/pidfile"
	"github.com/dev-kryptic/daemon/internal/server"
)

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
