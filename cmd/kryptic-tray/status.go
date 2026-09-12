package main

import "sync/atomic"

const (
	traySignedOut        = "signed_out"
	trayConnecting       = "connecting"
	trayAwaitingApproval = "awaiting_approval"
	trayConnected        = "connected"
)

var trayConnection atomic.Value

func setTrayConnection(kind string) {
	if kind == "" {
		kind = traySignedOut
	}
	trayConnection.Store(kind)
}

func currentTrayConnection() string {
	v, _ := trayConnection.Load().(string)
	if v == "" {
		return traySignedOut
	}
	return v
}

func trayDotRGB(kind string) (r, g, b uint8) {
	switch kind {
	case trayConnected:
		return 48, 209, 88
	case trayConnecting, trayAwaitingApproval:
		return 255, 159, 10
	default:
		return 142, 142, 147
	}
}

func trayStatusTitle(kind, email string) string {
	switch kind {
	case trayConnecting:
		return "🟠 Connecting…"
	case trayAwaitingApproval:
		return "🟠 Awaiting approval"
	case trayConnected:
		if email != "" {
			return "🟢 Connected · " + email
		}
		return "🟢 Connected"
	default:
		return "⚪ Signed out"
	}
}

type profileInfo struct {
	id           string
	email        string
	organization string
	active       bool
	signedIn     bool
}

func (p profileInfo) title() string {
	label := p.email
	if label == "" {
		label = p.id
	}
	if p.organization != "" {
		label += " · " + p.organization
	}
	if p.active {
		return "✓ " + label
	}
	if !p.signedIn {
		return label + " (signed out)"
	}
	return label
}

func parseProfiles(response map[string]any) []profileInfo {
	raw, _ := response["profiles"].([]any)
	out := make([]profileInfo, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		if id == "" {
			continue
		}
		email, _ := m["email"].(string)
		org, _ := m["organization"].(string)
		active, _ := m["active"].(bool)
		signedIn, _ := m["signedIn"].(bool)
		out = append(out, profileInfo{id, email, org, active, signedIn})
	}
	return out
}

func trayTooltip(kind string) string {
	switch kind {
	case trayConnecting:
		return "Kryptic: connecting"
	case trayAwaitingApproval:
		return "Kryptic: awaiting approval"
	case trayConnected:
		return "Kryptic: connected"
	default:
		return "Kryptic: signed out"
	}
}
