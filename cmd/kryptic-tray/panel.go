package main

import (
	"sync"

	"github.com/dev-kryptic/daemon/internal/manageui"
	"github.com/dev-kryptic/daemon/internal/server"
)

type panelState struct {
	mu              sync.Mutex
	connection      string
	email           string
	organization    string
	api             string
	authenticated   bool
	running         bool
	loginInProgress bool
	loginCode       string
	loginError      string
	updateTitle     string
	scanInProgress  bool
	profiles        []manageui.Profile
}

func (p *panelState) snapshot() manageui.Snapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	return manageui.Snapshot{
		Connection:      p.connection,
		ConnectionLabel: manageui.ConnectionLabel(p.connection, p.email),
		Email:           p.email,
		Organization:    p.organization,
		API:             p.api,
		Authenticated:   p.authenticated,
		Running:         p.running,
		CanLogin:        true,
		LoginInProgress: p.loginInProgress,
		LoginCode:       p.loginCode,
		LoginError:      p.loginError,
		UpdateTitle:     p.updateTitle,
		ScanInProgress:  p.scanInProgress,
		Profiles:        append([]manageui.Profile(nil), p.profiles...),
		Version:         server.Version,
	}
}

func (p *panelState) setStatus(connection, email, org, api string, authed, running bool, profiles []manageui.Profile) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.connection = connection
	p.email = email
	p.organization = org
	p.api = api
	p.authenticated = authed
	p.running = running
	p.profiles = profiles
}

func (p *panelState) setLogin(inProgress bool, code, err string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.loginInProgress = inProgress
	p.loginCode = code
	p.loginError = err
}

func (p *panelState) setUpdateTitle(title string) {
	p.mu.Lock()
	p.updateTitle = title
	p.mu.Unlock()
}

func (p *panelState) setScan(busy bool) {
	p.mu.Lock()
	p.scanInProgress = busy
	p.mu.Unlock()
}
