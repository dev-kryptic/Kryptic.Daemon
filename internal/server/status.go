package server

import (
	"errors"
	"time"

	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/applog"
	"github.com/dev-kryptic/daemon/internal/authstore"
)

func (s *Server) handleStatus() map[string]any {
	base := map[string]any{
		"ok": true, "authenticated": false, "daemonVersion": Version,
		"apiUrl": s.client.BaseURL, "orgKeyGranted": true,
		"connection": "signed_out",
	}
	attachProfiles(base)

	token, err := s.token()
	if err != nil {
		hasSession := !errors.Is(err, authstore.ErrNotLoggedIn)
		return s.statusFromFailure(base, hasSession, err)
	}

	me, err := s.client.Me(token)
	if err != nil && isUnauthorized(err) {
		s.invalidateAccessToken()
		token, retryErr := s.token()
		if retryErr != nil {
			err = retryErr
		} else {
			me, err = s.client.Me(token)
		}
	}
	if err != nil {
		applog.Error("daemon", "auth.me", err, "result=error")
		hasSession := !errors.Is(err, authstore.ErrNotLoggedIn)
		return s.statusFromFailure(base, hasSession, err)
	}

	s.mu.Lock()
	s.lastMe = me
	s.mu.Unlock()
	return statusFromMe(base, me)
}

func (s *Server) statusFromFailure(base map[string]any, hasSession bool, err error) map[string]any {
	if !sessionReadsAsSignedIn(hasSession, err) {
		s.logStatusThrottled("status.unauthenticated", err)
		s.mu.Lock()
		s.lastMe = nil
		s.mu.Unlock()
		return base
	}

	s.logStatusThrottled("status.degraded", err)
	s.mu.Lock()
	me := s.lastMe
	s.mu.Unlock()
	base["authenticated"] = true
	base["connection"] = "connected"
	if me != nil {
		return statusFromMe(base, me)
	}
	return base
}

func (s *Server) logStatusThrottled(event string, err error) {
	s.mu.Lock()
	due := time.Since(s.lastStatusLog) >= 30*time.Second
	if due {
		s.lastStatusLog = time.Now()
	}
	s.mu.Unlock()
	if due {
		applog.Error("daemon", event, err)
	}
}

func statusFromMe(base map[string]any, me *api.Me) map[string]any {
	orgKeyGranted := true
	if me.HasOrgKeyGrant != nil {
		orgKeyGranted = *me.HasOrgKeyGrant
	}

	base["authenticated"] = true
	base["email"] = me.Email
	base["organization"] = me.Organization
	base["orgKeyGranted"] = orgKeyGranted
	if !orgKeyGranted {
		base["connection"] = "awaiting_approval"
		notifyMissingOrgKey()
	} else {
		base["connection"] = "connected"
	}
	return base
}

func attachProfiles(base map[string]any) {
	store, err := authstore.LoadStore()
	if err != nil {
		base["profiles"] = []map[string]any{}
		return
	}
	list := make([]map[string]any, 0, len(store.Profiles))
	for _, p := range store.Profiles {
		list = append(list, map[string]any{
			"id":           p.ID,
			"email":        p.Email,
			"organization": p.Organization,
			"api":          p.API(),
			"active":       p.ID == store.ActiveID,
			"signedIn":     p.SignedIn(),
		})
	}
	base["profiles"] = list
	if store.ActiveID != "" {
		base["activeProfileId"] = store.ActiveID
	}
}

func (s *Server) pollOrgKeyGrant() {
	time.Sleep(2 * time.Second)
	for {
		s.handleStatus()
		time.Sleep(30 * time.Second)
	}
}
