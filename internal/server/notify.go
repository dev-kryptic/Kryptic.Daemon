package server

import "github.com/dev-kryptic/daemon/internal/notify"

func notifyMissingOrgKey() {
	notify.Alert("org-key", "Kryptic is signed in but cannot decrypt",
		"An admin must grant this device the organization key under Approvals. Until then no project secrets can be injected.")
}

func notifyProjectDenied(projectId, environment, message string) {
	label := projectId
	if environment != "" {
		label += "/" + environment
	}
	notify.Alert("project:"+label, "Kryptic cannot inject secrets", message)
}
