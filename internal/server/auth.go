package server

import (
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/applog"
	"github.com/dev-kryptic/daemon/internal/authstore"
	"github.com/dev-kryptic/daemon/internal/config"
)

func (s *Server) ResetAuth() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accessToken = ""
	s.tokenExpiry = time.Time{}
	s.lastMe = nil
	s.cache = map[string]cacheEntry{}
}

func (s *Server) handleResetAuth() okReply {
	s.ResetAuth()
	log.Printf("signed out - dropped the access token and secrets cache")
	applog.Event("daemon", "auth.reset")
	return okReply{V: protocolVersion, OK: true}
}

func (s *Server) handleFlush() flushReply {
	s.mu.Lock()
	cleared := len(s.cache)
	s.cache = map[string]cacheEntry{}
	s.mu.Unlock()

	log.Printf("secrets cache flushed (%d bundle(s) dropped)", cleared)
	applog.Event("daemon", "cache.flush", "cleared="+strconv.Itoa(cleared))
	return flushReply{V: protocolVersion, OK: true, Cleared: cleared}
}

func (s *Server) handleSwitchProfile(req request) any {
	id := strings.TrimSpace(req.ProfileId)
	if id == "" {
		return fail("internal", "profileId is required")
	}
	if err := authstore.Switch(id); err != nil {
		if errors.Is(err, authstore.ErrUnknownProfile) {
			return fail("internal", "unknown profile")
		}
		return fail("internal", err.Error())
	}
	s.ResetAuth()
	s.applyActiveAPI()
	applog.Event("daemon", "auth.profile.switch")
	return s.handleStatus()
}

func (s *Server) handleSetAPI(req request) any {
	raw := strings.TrimSpace(req.API)
	if raw == "" {
		raw = config.DefaultAPI
	}
	if err := authstore.SetActiveAPI(raw); err != nil {
		return fail("internal", err.Error())
	}
	s.ResetAuth()
	s.applyActiveAPI()
	applog.Event("daemon", "config.api")
	return s.handleStatus()
}

func (s *Server) handleDeleteProfile(req request) any {
	id := strings.TrimSpace(req.ProfileId)
	if id == "" {
		return fail("internal", "profileId is required")
	}
	if err := loginRemoveProfile(id); err != nil {
		if errors.Is(err, authstore.ErrUnknownProfile) {
			return fail("internal", "unknown profile")
		}
		return fail("internal", err.Error())
	}
	s.ResetAuth()
	s.applyActiveAPI()
	applog.Event("daemon", "auth.profile.delete")
	return s.handleStatus()
}

func loginRemoveProfile(idOrEmail string) error {
	store, err := authstore.LoadStore()
	if err != nil {
		return err
	}
	profile, ok := authstore.FindProfile(store, idOrEmail)
	if !ok {
		return authstore.ErrUnknownProfile
	}
	if profile.RefreshToken != "" {
		client := api.NewClientFor(profile.API())
		if tokens, refreshErr := client.Refresh(profile.RefreshToken); refreshErr == nil {
			_ = client.RevokeDevice(tokens.AccessToken)
			_ = client.Logout(tokens.AccessToken)
		}
	}
	return authstore.Remove(profile.ID)
}

func (s *Server) applyActiveAPI() {
	next := authstore.ResolvedAPI()
	if s.client.BaseURL == next {
		return
	}
	s.client.BaseURL = next
}

func (s *Server) SetBaseURL(baseURL string) {
	s.ResetAuth()
	s.client.BaseURL = baseURL
}

func (s *Server) SyncActiveAPI() {
	s.applyActiveAPI()
}

func (s *Server) invalidateAccessToken() {
	s.mu.Lock()
	s.accessToken = ""
	s.tokenExpiry = time.Time{}
	s.mu.Unlock()
}

func (s *Server) cachedAccessToken() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.accessToken != "" && time.Until(s.tokenExpiry) > time.Minute {
		return s.accessToken, true
	}
	return "", false
}

func (s *Server) token() (string, error) {
	if token, ok := s.cachedAccessToken(); ok {
		return token, nil
	}

	var token string
	err := authstore.WithLock(func() error {
		if cached, ok := s.cachedAccessToken(); ok {
			token = cached
			return nil
		}

		session, err := authstore.LoadSession()
		if err != nil {
			return err
		}
		if session.RefreshToken == "" {
			return authstore.ErrNotLoggedIn
		}

		tokens, err := s.client.Refresh(session.RefreshToken)
		if isInvalidRefresh(err) {
			session, err = authstore.LoadSession()
			if err != nil {
				return err
			}
			tokens, err = s.client.Refresh(session.RefreshToken)
		}
		if err != nil {
			applog.Error("daemon", "auth.refresh", err, "result=error")
			return err
		}

		session.RefreshToken = tokens.RefreshToken
		if err := authstore.SaveSession(session); err != nil {
			applog.Error("daemon", "auth.save", err, "result=error")
			return err
		}

		s.mu.Lock()
		s.accessToken = tokens.AccessToken
		s.tokenExpiry = time.Now().Add(time.Duration(tokens.ExpiresInSeconds) * time.Second)
		token = s.accessToken
		s.mu.Unlock()
		applog.Event("daemon", "auth.refresh", "result=ok")
		return nil
	})
	return token, err
}
