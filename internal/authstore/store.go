// Package authstore keeps the daemon's session - the refresh token and the
// device's sealed-box key pair - in the OS credential store: the user Keychain
// on macOS (via /usr/bin/security), the Credential Manager on Windows
// (advapi32), and libsecret on Linux (via secret-tool). When the platform
// store is unavailable (e.g. a headless Linux box without a Secret Service),
// it falls back to a 0600 file under the user config directory. Access tokens
// are never persisted - they live in daemon memory only.
//
// One OS user can keep several profiles (personal + work, cloud + self-host).
// Each profile has its own refresh token, device keys, and config (including
// the Daemon BFF URL). Switching the active profile does not sign the others
// out and does not share org-key grants.
package authstore

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dev-kryptic/daemon/internal/config"
)

var (
	ErrNotLoggedIn    = errors.New("not logged in - run `kryptic login` first")
	ErrUnknownProfile = errors.New("unknown profile")
)

var (
	fileOnlyMu sync.Mutex
	fileOnly   bool
)

// Session is the active profile as the rest of the daemon sees it. The device
// private key opens the org-key grant sealed to this device (end-to-end
// encryption); it exists nowhere else, so wiping the profile also revokes
// that device's ability to decrypt.
type Session struct {
	ID               string        `json:"id,omitempty"`
	Email            string        `json:"email,omitempty"`
	Organization     string        `json:"organization,omitempty"`
	RefreshToken     string        `json:"refreshToken"`
	DevicePublicKey  string        `json:"devicePublicKey,omitempty"`
	DevicePrivateKey string        `json:"devicePrivateKey,omitempty"`
	Config           ProfileConfig `json:"config,omitempty"`
}

// ProfileConfig is persisted per account: cloud vs self-host, and room for
// later knobs. Empty API means the install default (config.json / hosted).
type ProfileConfig struct {
	API string `json:"api,omitempty"`
}

// Profile is one saved account on this install.
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

// Store is the persisted multi-profile envelope.
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

func FindByIdentity(store Store, email, org, api string) (Profile, bool) {
	email = strings.ToLower(strings.TrimSpace(email))
	org = strings.TrimSpace(org)
	if email == "" {
		return Profile{}, false
	}
	wantAPI := CanonicalAPI(api)
	for _, p := range store.Profiles {
		if strings.EqualFold(p.Email, email) && strings.TrimSpace(p.Organization) == org && p.API() == wantAPI {
			return p, true
		}
	}
	return Profile{}, false
}

func CanonicalAPI(raw string) string {
	url, _ := config.Resolve(raw)
	return url
}

func ResolvedAPI() string {
	store, err := LoadStore()
	if err == nil {
		if p, ok := store.Active(); ok {
			return p.API()
		}
	}
	url, _ := config.API()
	return url
}

func FindProfile(store Store, idOrEmail string) (Profile, bool) {
	want := strings.TrimSpace(idOrEmail)
	if want == "" {
		return Profile{}, false
	}
	for _, p := range store.Profiles {
		if p.ID == want || strings.EqualFold(p.Email, want) {
			return p, true
		}
	}
	return Profile{}, false
}

// SaveSession stores the active profile, falling back to the config-dir file
// when the platform store is unavailable. Extra profiles are kept.
func SaveSession(session Session) error {
	store, err := LoadStore()
	if err != nil && !errors.Is(err, ErrNotLoggedIn) {
		return err
	}
	p, ok := store.Active()
	if !ok {
		id := session.ID
		if id == "" {
			id = newProfileID()
		}
		p = Profile{ID: id}
	} else if session.ID != "" && session.ID != p.ID {
		p = Profile{ID: session.ID}
	}
	p.RefreshToken = session.RefreshToken
	if session.DevicePublicKey != "" {
		p.DevicePublicKey = session.DevicePublicKey
	}
	if session.DevicePrivateKey != "" {
		p.DevicePrivateKey = session.DevicePrivateKey
	}
	if session.Email != "" {
		p.Email = session.Email
	}
	if session.Organization != "" {
		p.Organization = session.Organization
	}
	if session.Config.API != "" {
		p.Config.API = session.Config.API
	}
	store.put(p)
	return SaveStore(store)
}

// LoadSession reads the active profile, preferring the platform credential store.
func LoadSession() (Session, error) {
	store, err := LoadStore()
	if err != nil {
		return Session{}, err
	}
	p, ok := store.Active()
	if !ok || (p.RefreshToken == "" && p.DevicePrivateKey == "") {
		return Session{}, ErrNotLoggedIn
	}
	return p.Session(), nil
}

func LoadStore() (Store, error) {
	raw, err := loadRaw()
	if err != nil {
		return Store{}, err
	}
	store, err := decodeStore(raw)
	if err != nil {
		return Store{}, err
	}
	if backfillProfileAPIs(&store) {
		_ = SaveStore(store)
	}
	return store, nil
}

func backfillProfileAPIs(store *Store) bool {
	install := config.InstallDefault()
	changed := false
	for i := range store.Profiles {
		if strings.TrimSpace(store.Profiles[i].Config.API) != "" {
			continue
		}
		store.Profiles[i].Config.API = install
		changed = true
	}
	return changed
}

func SaveStore(store Store) error {
	if len(store.Profiles) == 0 {
		return Clear()
	}
	if store.ActiveID == "" {
		store.ActiveID = store.Profiles[0].ID
	}
	data, err := json.Marshal(store)
	if err != nil {
		return err
	}
	return saveRaw(string(data))
}

func decodeSession(raw string) (Session, error) {
	store, err := decodeStore(raw)
	if err != nil {
		return Session{}, err
	}
	p, ok := store.Active()
	if !ok || (p.RefreshToken == "" && p.DevicePrivateKey == "") {
		return Session{}, ErrNotLoggedIn
	}
	return p.Session(), nil
}

func decodeStore(raw string) (Store, error) {
	var envelope Store
	if json.Unmarshal([]byte(raw), &envelope) == nil && len(envelope.Profiles) > 0 {
		for i := range envelope.Profiles {
			if envelope.Profiles[i].ID == "" {
				envelope.Profiles[i].ID = newProfileID()
			}
		}
		if envelope.ActiveID == "" {
			envelope.ActiveID = envelope.Profiles[0].ID
		}
		return envelope, nil
	}

	var session Session
	if json.Unmarshal([]byte(raw), &session) != nil {
		return Store{}, ErrNotLoggedIn
	}
	if session.RefreshToken == "" && session.DevicePrivateKey == "" {
		return Store{}, ErrNotLoggedIn
	}
	id := session.ID
	if id == "" {
		id = "default"
	}
	return Store{
		ActiveID: id,
		Profiles: []Profile{{
			ID:               id,
			Email:            session.Email,
			Organization:     session.Organization,
			RefreshToken:     session.RefreshToken,
			DevicePublicKey:  session.DevicePublicKey,
			DevicePrivateKey: session.DevicePrivateKey,
		}},
	}, nil
}

// HasDeviceKeys is true when this OS user still has a durable key pair on the
// active profile.
func HasDeviceKeys(session Session) bool {
	return session.DevicePublicKey != "" && session.DevicePrivateKey != ""
}

// SaveLoggedIn writes a completed login. Matching email+org updates that
// profile (and keeps its keys). A new identity becomes a new profile when
// add is true or the current slot already belongs to someone else.
func SaveLoggedIn(session Session, add bool) error {
	store, err := LoadStore()
	if err != nil && !errors.Is(err, ErrNotLoggedIn) {
		return err
	}

	if existing, ok := FindByIdentity(store, session.Email, session.Organization, session.Config.API); ok {
		existing.RefreshToken = session.RefreshToken
		existing.Email = session.Email
		existing.Organization = session.Organization
		if session.Config.API != "" {
			existing.Config.API = session.Config.API
		}
		if !existing.HasDeviceKeys() {
			existing.DevicePublicKey = session.DevicePublicKey
			existing.DevicePrivateKey = session.DevicePrivateKey
		}
		store.put(existing)
		return SaveStore(store)
	}

	current, hasCurrent := store.Active()
	sameIdentity := hasCurrent && strings.EqualFold(current.Email, session.Email) &&
		current.Organization == session.Organization &&
		current.API() == CanonicalAPI(session.Config.API)
	reuseCurrent := hasCurrent && !add && (current.Email == "" || sameIdentity)
	if reuseCurrent {
		current.RefreshToken = session.RefreshToken
		current.Email = session.Email
		current.Organization = session.Organization
		if session.Config.API != "" {
			current.Config.API = session.Config.API
		}
		if session.DevicePublicKey != "" {
			current.DevicePublicKey = session.DevicePublicKey
			current.DevicePrivateKey = session.DevicePrivateKey
		}
		store.put(current)
		return SaveStore(store)
	}

	id := session.ID
	if id == "" {
		id = newProfileID()
	}
	store.put(Profile{
		ID:               id,
		Email:            session.Email,
		Organization:     session.Organization,
		RefreshToken:     session.RefreshToken,
		DevicePublicKey:  session.DevicePublicKey,
		DevicePrivateKey: session.DevicePrivateKey,
		Config:           session.Config,
	})
	return SaveStore(store)
}

// Switch makes id (or email) the active profile. Other profiles stay signed in.
func Switch(idOrEmail string) error {
	store, err := LoadStore()
	if err != nil {
		return err
	}
	p, ok := FindProfile(store, idOrEmail)
	if !ok {
		return ErrUnknownProfile
	}
	store.ActiveID = p.ID
	return SaveStore(store)
}

// ClearRefresh drops the active profile's session token and keeps its keys.
func ClearRefresh() error {
	store, err := LoadStore()
	if err != nil {
		return Clear()
	}
	p, ok := store.Active()
	if !ok {
		return Clear()
	}
	p.RefreshToken = ""
	store.put(p)
	return SaveStore(store)
}

// ClearAllRefresh signs every profile out and keeps their keys.
func ClearAllRefresh() error {
	store, err := LoadStore()
	if err != nil {
		return nil
	}
	for i := range store.Profiles {
		store.Profiles[i].RefreshToken = ""
	}
	return SaveStore(store)
}

// RemoveActive deletes the active profile (keys included). Another profile
// becomes active when one remains.
func RemoveActive() error {
	store, err := LoadStore()
	if err != nil {
		return Clear()
	}
	p, ok := store.Active()
	if !ok {
		return Clear()
	}
	rest := make([]Profile, 0, len(store.Profiles))
	for _, existing := range store.Profiles {
		if existing.ID != p.ID {
			rest = append(rest, existing)
		}
	}
	if len(rest) == 0 {
		return Clear()
	}
	store.Profiles = rest
	store.ActiveID = rest[0].ID
	return SaveStore(store)
}

// Remove deletes one profile (keys included). If it was active, another
// profile becomes active.
func Remove(idOrEmail string) error {
	store, err := LoadStore()
	if err != nil {
		return ErrUnknownProfile
	}
	p, ok := FindProfile(store, idOrEmail)
	if !ok {
		return ErrUnknownProfile
	}
	rest := make([]Profile, 0, len(store.Profiles))
	for _, existing := range store.Profiles {
		if existing.ID != p.ID {
			rest = append(rest, existing)
		}
	}
	if len(rest) == 0 {
		return Clear()
	}
	store.Profiles = rest
	if store.ActiveID == p.ID {
		store.ActiveID = rest[0].ID
	}
	return SaveStore(store)
}

// SetActiveAPI writes the Daemon BFF on the active profile and drops that
// profile's refresh token. Other profiles are left alone.
func SetActiveAPI(raw string) error {
	normalized, err := config.NormalizeAPI(raw)
	if err != nil {
		if strings.TrimSpace(raw) != "" {
			return err
		}
		normalized = config.DefaultAPI
	}
	store, err := LoadStore()
	if err != nil {
		if !errors.Is(err, ErrNotLoggedIn) {
			return err
		}
		return config.SetAPI(normalized)
	}
	p, ok := store.Active()
	if !ok {
		return config.SetAPI(normalized)
	}
	if p.API() != normalized {
		p.RefreshToken = ""
	}
	p.Config.API = normalized
	store.replace(p)
	return SaveStore(store)
}

func loadRaw() (string, error) {
	if !fileOnly {
		raw, err := platformLoad()
		if err == nil && raw != "" {
			return raw, nil
		}
	}
	return loadFile()
}

func saveRaw(data string) error {
	if !fileOnly {
		if err := platformSave(data); err == nil {
			if path, pathErr := filePath(); pathErr == nil {
				_ = os.Remove(path)
			}
			return nil
		}
	}
	return saveFile(data)
}

// Clear removes every profile from both the platform store and the file
// fallback (whichever holds it).
func Clear() error {
	if !fileOnly {
		platformClear()
	}
	if path, err := filePath(); err == nil {
		_ = os.Remove(path)
	}
	return nil
}

// ---------- file fallback ----------

func filePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "kryptic", "session"), nil
}

func saveFile(refreshToken string) error {
	path, err := filePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(refreshToken), 0o600)
}

func loadFile() (string, error) {
	path, err := filePath()
	if err != nil {
		return "", ErrNotLoggedIn
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ErrNotLoggedIn
	}
	return strings.TrimSpace(string(data)), nil
}
