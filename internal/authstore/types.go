package authstore

import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/dev-kryptic/daemon/internal/config"
)

var (
	ErrNotLoggedIn    = errors.New("not logged in - run `kryptic login` first")
	ErrUnknownProfile = errors.New("unknown profile")
)

type Session struct {
	ID               string        `json:"id,omitempty"`
	Email            string        `json:"email,omitempty"`
	Organization     string        `json:"organization,omitempty"`
	RefreshToken     string        `json:"refreshToken"`
	DevicePublicKey  string        `json:"devicePublicKey,omitempty"`
	DevicePrivateKey string        `json:"devicePrivateKey,omitempty"`
	Config           ProfileConfig `json:"config,omitempty"`
}

type ProfileConfig struct {
	API string `json:"api,omitempty"`
}

type Profile struct {
	ID               string        `json:"id"`
	Email            string        `json:"email,omitempty"`
	Organization     string        `json:"organization,omitempty"`
	RefreshToken     string        `json:"refreshToken,omitempty"`
	DevicePublicKey  string        `json:"devicePublicKey,omitempty"`
	DevicePrivateKey string        `json:"devicePrivateKey,omitempty"`
	Config           ProfileConfig `json:"config,omitempty"`
}

func (p Profile) SignedIn() bool {
	return p.RefreshToken != ""
}

func (p Profile) Session() Session {
	return Session{
		ID:               p.ID,
		Email:            p.Email,
		Organization:     p.Organization,
		RefreshToken:     p.RefreshToken,
		DevicePublicKey:  p.DevicePublicKey,
		DevicePrivateKey: p.DevicePrivateKey,
		Config:           p.Config,
	}
}

func (p Profile) API() string {
	url, _ := config.Resolve(p.Config.API)
	return url
}

func (s Session) API() string {
	url, _ := config.Resolve(s.Config.API)
	return url
}

func (p Profile) HasDeviceKeys() bool {
	return p.DevicePublicKey != "" && p.DevicePrivateKey != ""
}

type Store struct {
	ActiveID string    `json:"activeId"`
	Profiles []Profile `json:"profiles"`
}

func (s Store) Active() (Profile, bool) {
	if s.ActiveID != "" {
		for _, p := range s.Profiles {
			if p.ID == s.ActiveID {
				return p, true
			}
		}
	}
	if len(s.Profiles) == 1 {
		return s.Profiles[0], true
	}
	return Profile{}, false
}

func (s *Store) replace(p Profile) {
	for i, existing := range s.Profiles {
		if existing.ID == p.ID {
			s.Profiles[i] = p
			return
		}
	}
	s.Profiles = append(s.Profiles, p)
}

func (s *Store) put(p Profile) {
	s.replace(p)
	s.ActiveID = p.ID
}

func newProfileID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "profile"
	}
	return hex.EncodeToString(b[:])
}
