// Package server implements the local socket side of daemon/PROTOCOL.md v1:
// SDKs connect, send one NDJSON request ("secrets" or "status"), get one reply.
// Secrets are cached in memory for 5 minutes and never touch disk.
//
// Bundles arrive end-to-end encrypted: ciphertext envelopes plus the org key
// sealed to this device. The daemon unwraps the org key with the device
// private key (stored at login) and decrypts every envelope locally - the
// platform never sees a plaintext secret.
package server

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/dev-kryptic/Kryptic.Encryption.Go/envelope"
	"github.com/dev-kryptic/Kryptic.Encryption.Go/sealedbox"
	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/applog"
	"github.com/dev-kryptic/daemon/internal/authstore"
	"github.com/dev-kryptic/daemon/internal/ipc"
	"github.com/dev-kryptic/daemon/internal/notify"
)

const (
	protocolVersion = 1
	cacheTTL        = 5 * time.Minute
)

// Version is the daemon version reported to the platform and compared by
// `kryptic update`. Release builds override it with:
// -ldflags "-X github.com/dev-kryptic/daemon/internal/server.Version=x.y.z"
var Version = "1.0.0"

// secretPair is what the local protocol serves to SDKs: already decrypted.
type secretPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type cacheEntry struct {
	secrets   []secretPair
	fetchedAt time.Time
}

type Server struct {
	client *api.Client

	mu            sync.Mutex
	accessToken   string
	tokenExpiry   time.Time
	cache         map[string]cacheEntry // "projectId/environment" -> bundle
	lastMe        *api.Me
	lastStatusLog time.Time
}

func New(client *api.Client) *Server {
	return &Server{client: client, cache: map[string]cacheEntry{}}
}

// Run listens on the local socket (unix) or named pipe (Windows) until the process ends.
func (s *Server) Run() error {
	listener, err := ipc.Listen()
	if err != nil {
		return err
	}
	defer listener.Close()

	log.Printf("kryptic daemon %s listening on %s (api: %s)", Version, ipc.Endpoint(), s.client.BaseURL)
	applog.Event("daemon", "start", "version="+Version)
	applog.Event("daemon", "listen")

	go s.pollOrgKeyGrant()

	for {
		connection, err := listener.Accept()
		if err != nil {
			return err
		}
		go s.handle(connection)
	}
}

type request struct {
	V           int    `json:"v"`
	Type        string `json:"type"`
	ProjectId   string `json:"projectId"`
	Environment string `json:"environment"`
}

func (s *Server) handle(connection net.Conn) {
	defer connection.Close()

	// Authenticate the caller by OS credentials before reading a single byte:
	// only a process running as the same user may pull secrets. An unauthorized
	// peer is dropped silently rather than engaged with a reply.
	if err := ipc.CheckPeer(connection); err != nil {
		log.Printf("denied connection: %v", err)
		applog.Error("daemon", "ipc.denied", err)
		return
	}

	_ = connection.SetDeadline(time.Now().Add(10 * time.Second))

	line, err := bufio.NewReader(connection).ReadBytes('\n')
	if err != nil {
		return
	}

	var req request
	if json.Unmarshal(line, &req) != nil {
		s.reply(connection, errorResponse("internal", "invalid request"))
		return
	}
	if req.V != protocolVersion {
		s.reply(connection, errorResponse("unsupported_version", "this daemon speaks protocol v1"))
		return
	}

	switch req.Type {
	case "secrets":
		s.reply(connection, s.handleSecrets(req))
	case "status":
		s.reply(connection, s.handleStatus())
	case "flush":
		s.reply(connection, s.handleFlush())
	case "reset-auth":
		s.reply(connection, s.handleResetAuth())
	default:
		s.reply(connection, errorResponse("internal", "unknown request type"))
	}
}

func (s *Server) reply(connection net.Conn, payload map[string]any) {
	payload["v"] = protocolVersion
	data, _ := json.Marshal(payload)
	_, _ = connection.Write(append(data, '\n'))
}

func errorResponse(code, message string) map[string]any {
	return map[string]any{"ok": false, "error": code, "message": message}
}

func (s *Server) handleSecrets(req request) map[string]any {
	cacheKey := req.ProjectId + "/" + req.Environment

	s.mu.Lock()
	if entry, hit := s.cache[cacheKey]; hit && time.Since(entry.fetchedAt) < cacheTTL {
		s.mu.Unlock()
		return map[string]any{"ok": true, "secrets": entry.secrets}
	}
	s.mu.Unlock()

	token, err := s.token()
	if err != nil {
		applog.Error("daemon", "bundle.auth", err)
		return errorResponse("not_authenticated", "run `kryptic login` first")
	}

	bundle, err := s.client.Bundle(token, req.ProjectId, req.Environment)
	if err != nil {
		var apiError *api.APIError
		if ok := asAPIError(err, &apiError); ok {
			applog.Error("daemon", "bundle.fetch", err, fmt.Sprintf("status=%d", apiError.Status))
			switch apiError.Status {
			case 401:
				return errorResponse("not_authenticated", "session revoked - run `kryptic login`")
			case 403:
				message := "you have no access to this environment"
				if apiError.Message != "" {
					message = apiError.Message
				}
				if strings.Contains(strings.ToLower(message), "organization key") {
					notifyMissingOrgKey()
				} else {
					notifyProjectDenied(req.ProjectId, req.Environment, message)
				}
				return errorResponse("access_denied", message)
			case 404:
				if isUnknownEnvironment(apiError) {
					return errorResponse("unknown_environment", apiError.Message)
				}
				return errorResponse("unknown_project", apiError.Message)
			}
		}
		return errorResponse("internal", err.Error())
	}

	secrets, err := decryptBundle(bundle)
	if err != nil {
		log.Printf("bundle decrypt failed for %s/%s", req.ProjectId, req.Environment)
		applog.Event("daemon", "bundle.decrypt", "result=error")
		// PROTOCOL.md: no org-key grant (missing, stale, or wrong device key)
		// is access_denied so packages skip the same way as a project 403.
		if isGrantFailure(err) {
			notifyMissingOrgKey()
			return errorResponse("access_denied", err.Error())
		}
		return errorResponse("internal", err.Error())
	}

	s.mu.Lock()
	s.cache[cacheKey] = cacheEntry{secrets: secrets, fetchedAt: time.Now()}
	s.mu.Unlock()

	log.Printf("served %d secrets for %s/%s (decrypted locally)", len(secrets), req.ProjectId, req.Environment)
	applog.Event("daemon", "bundle.ok", fmt.Sprintf("count=%d", len(secrets)))
	return map[string]any{"ok": true, "secrets": secrets}
}

// decryptBundle unwraps the org key with this device's private key and opens
// every envelope. All crypto happens here, on the developer's machine.
func decryptBundle(bundle *api.Bundle) ([]secretPair, error) {
	session, err := authstore.LoadSession()
	if err != nil {
		return nil, err
	}
	if session.DevicePrivateKey == "" {
		return nil, fmt.Errorf("this session has no device key - run `kryptic login` again")
	}

	enc := base64.RawURLEncoding
	private, err := enc.DecodeString(session.DevicePrivateKey)
	if err != nil {
		return nil, fmt.Errorf("stored device key is corrupt - run `kryptic login` again")
	}
	public, err := enc.DecodeString(session.DevicePublicKey)
	if err != nil {
		return nil, fmt.Errorf("stored device key is corrupt - run `kryptic login` again")
	}

	sealed, err := sealedbox.Parse(bundle.WrappedOrgKey)
	if err != nil {
		return nil, fmt.Errorf("invalid wrapped org key: %w", err)
	}
	orgKey, err := sealedbox.Open(sealedbox.KeyPair{Public: public, Private: private}, sealed)
	if err != nil {
		return nil, fmt.Errorf("could not unwrap the org key - the device grant may be stale, run `kryptic login` again")
	}

	secrets := make([]secretPair, 0, len(bundle.Secrets))
	for _, entry := range bundle.Secrets {
		associatedData := envelope.SecretContext(entry.DefinitionId, entry.EnvironmentId)
		plaintext, err := envelope.Open(orgKey, entry.Envelope, associatedData)
		if err != nil {
			return nil, fmt.Errorf("decrypt %q: %w", entry.Key, err)
		}
		secrets = append(secrets, secretPair{Key: entry.Key, Value: string(plaintext)})
	}
	return secrets, nil
}

// ResetAuth drops the in-memory access token and cached bundles - called by the
// tray apps after sign-out so state clears immediately instead of at token expiry.
func (s *Server) ResetAuth() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accessToken = ""
	s.tokenExpiry = time.Time{}
	s.lastMe = nil
	s.cache = map[string]cacheEntry{}
}

// handleResetAuth is `reset-auth` on the socket: ResetAuth for an out-of-process
// caller. `kryptic logout` sends it so a running daemon reflects the sign-out
// immediately instead of serving its cached token until expiry (up to 15 min).
func (s *Server) handleResetAuth() map[string]any {
	s.ResetAuth()
	log.Printf("signed out - dropped the access token and secrets cache")
	applog.Event("daemon", "auth.reset")
	return map[string]any{"ok": true}
}

// handleFlush drops every cached bundle so the next SDK request refetches from the
// platform - how a user picks up an admin's secret fix without waiting out the TTL.
func (s *Server) handleFlush() map[string]any {
	s.mu.Lock()
	cleared := len(s.cache)
	s.cache = map[string]cacheEntry{}
	s.mu.Unlock()

	log.Printf("secrets cache flushed (%d bundle(s) dropped)", cleared)
	applog.Event("daemon", "cache.flush", fmt.Sprintf("cleared=%d", cleared))
	return map[string]any{"ok": true, "cleared": cleared}
}

func (s *Server) handleStatus() map[string]any {
	base := map[string]any{
		"ok": true, "authenticated": false, "daemonVersion": Version,
		"apiUrl": s.client.BaseURL, "orgKeyGranted": true,
	}

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
		notifyMissingOrgKey()
	}
	return base
}

// pollOrgKeyGrant keeps the missing-grant OS notification alive even when no SDK
// or tray is asking for status (for example `kryptic start` with no menu bar).
func (s *Server) pollOrgKeyGrant() {
	time.Sleep(2 * time.Second)
	for {
		s.handleStatus()
		time.Sleep(30 * time.Second)
	}
}

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

// SetBaseURL points this process at a different Daemon BFF and drops cached
// auth so the next request uses the new host.
func (s *Server) SetBaseURL(baseURL string) {
	s.ResetAuth()
	s.client.BaseURL = baseURL
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

// token returns a live access token, refreshing through the stored (rotating)
// refresh token when the current one is close to expiry. The session lock
// serializes this with CLI commands so two processes cannot spend the same
// rotating token. The in-process mutex is not held during the HTTP call.
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

		tokens, err := s.client.Refresh(session.RefreshToken)
		if isInvalidRefresh(err) {
			// Another process may have just rotated; reload and try once.
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

func isUnknownEnvironment(err *api.APIError) bool {
	if err.Code == "unknown_environment" {
		return true
	}
	return strings.Contains(strings.ToLower(err.Message), "environment")
}

func isGrantFailure(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unwrap the org key") ||
		strings.Contains(msg, "no device key") ||
		strings.Contains(msg, "invalid wrapped org key") ||
		strings.Contains(msg, "device key is corrupt")
}

func asAPIError(err error, target **api.APIError) bool {
	e, ok := err.(*api.APIError)
	if ok {
		*target = e
	}
	return ok
}
