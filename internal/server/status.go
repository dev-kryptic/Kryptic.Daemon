package server

import (
	"errors"
	"time"

	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/applog"
	"github.com/dev-kryptic/daemon/internal/authstore"
)

func (s *Server) handleStatus() statusReply {
	base := statusReply{
		V:             protocolVersion,
		OK:            true,
		DaemonVersion: Version,
		APIUrl:        s.client.BaseURL,
		OrgKeyGranted: true,
		Connection:    "signed_out",
		Profiles:      []profileInfo{},
	}
	attachProfiles(&base)

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

func (s *Server) statusFromFailure(base statusReply, hasSession bool, err error) statusReply {
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
	base.Authenticated = true
	base.Connection = "connected"
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

func statusFromMe(base statusReply, me *api.Me) statusReply {
	orgKeyGranted := true
	if me.HasOrgKeyGrant != nil {
		orgKeyGranted = *me.HasOrgKeyGrant
	}

	base.Authenticated = true
	base.Email = me.Email
	base.Organization = me.Organization
	base.OrgKeyGranted = orgKeyGranted
	if !orgKeyGranted {
		base.Connection = "awaiting_approval"
		notifyMissingOrgKey()
	} else {
		base.Connection = "connected"
	}
	return base
}

func attachProfiles(base *statusReply) {
	store, err := authstore.LoadStore()
	if err != nil {
		return
	}
	list := make([]profileInfo, 0, len(store.Profiles))
	for _, p := range store.Profiles {
		list = append(list, profileInfo{
			ID:           p.ID,
			Email:        p.Email,
			Organization: p.Organization,
			API:          p.API(),
			Active:       p.ID == store.ActiveID,
			SignedIn:     p.SignedIn(),
		})
	}
	base.Profiles = list
	base.ActiveProfileId = store.ActiveID
}

func (s *Server) pollOrgKeyGrant() {
	time.Sleep(2 * time.Second)
	for {
		s.handleStatus()
		time.Sleep(30 * time.Second)
	}
}
