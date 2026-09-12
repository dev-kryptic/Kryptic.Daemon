package manageui

import "github.com/dev-kryptic/daemon/internal/server"

// Snapshot is the live state Open Kryptic paints.
type Snapshot struct {
	Connection      string
	ConnectionLabel string
	Email           string
	Organization    string
	API             string
	Authenticated   bool
	Running         bool
	CanLogin        bool
	LoginInProgress bool
	LoginCode       string
	LoginError      string
	UpdateTitle     string
	ScanInProgress  bool
	Profiles        []Profile
	Version         string
}

type Profile struct {
	ID           string
	Email        string
	Organization string
	API          string
	Active       bool
	SignedIn     bool
}

func (p Profile) Title() string {
	label := p.Email
	if label == "" {
		label = p.ID
	}
	if p.Organization != "" {
		label += " · " + p.Organization
	}
	if host := HostLabel(p.API); host != "" {
		label += " · " + host
	}
	if !p.SignedIn {
		label += " (signed out)"
	}
	if p.Active {
		return "✓ " + label
	}
	return label
}

func ConnectionLabel(kind, email string) string {
	switch kind {
	case "connecting":
		return "Connecting…"
	case "awaiting_approval":
		return "Awaiting approval"
	case "connected":
		return "Connected"
	default:
		return "Signed out"
	}
}

func DefaultVersion() string {
	return server.Version
}

func ProfilesFromStatus(raw any) []Profile {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]Profile, 0, len(items))
	for _, item := range items {
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
		api, _ := m["api"].(string)
		out = append(out, Profile{
			ID: id, Email: email, Organization: org, API: api,
			Active: active, SignedIn: signedIn,
		})
	}
	return out
}

type Handlers struct {
	Snapshot func() Snapshot
	Action   func(name, arg string)
}

func (h Handlers) fire(name, arg string) {
	if h.Action != nil {
		go h.Action(name, arg)
	}
}

func (h Handlers) snap() Snapshot {
	if h.Snapshot == nil {
		return Snapshot{ConnectionLabel: "Signed out", UpdateTitle: "Check for Updates"}
	}
	return h.Snapshot()
}
