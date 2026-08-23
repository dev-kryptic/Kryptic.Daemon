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
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/dev-kryptic/Kryptic.Encryption.Go/envelope"
	"github.com/dev-kryptic/Kryptic.Encryption.Go/sealedbox"
	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/authstore"
	"github.com/dev-kryptic/daemon/internal/ipc"
)

const (
	protocolVersion = 1
	cacheTTL        = 5 * time.Minute
)

// Version is the daemon version reported to the platform and compared by
// `kryptic update`. Release builds override it with:
// -ldflags "-X github.com/dev-kryptic/daemon/internal/server.Version=x.y.z"
var Version = "0.1.0"

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

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
	cache       map[string]cacheEntry // "projectId/environment" -> bundle
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
		return errorResponse("not_authenticated", "run `kryptic login` first")
	}

	bundle, err := s.client.Bundle(token, req.ProjectId, req.Environment)
	if err != nil {
		var apiError *api.APIError
		if ok := asAPIError(err, &apiError); ok {
			switch apiError.Status {
			case 401:
				return errorResponse("not_authenticated", "session revoked - run `kryptic login`")
			case 403:
				message := "you have no access to this environment"
				// The platform also 403s when the device has no org-key grant
				// yet (an admin must approve it) - surface that reason.
				if apiError.Message != "" {
					message = apiError.Message
				}
				return errorResponse("access_denied", message)
			case 404:
				return errorResponse("unknown_project", apiError.Message)
			}
		}
		return errorResponse("internal", err.Error())
	}

	secrets, err := decryptBundle(bundle)
	if err != nil {
		log.Printf("bundle decrypt failed for %s/%s: %v", req.ProjectId, req.Environment, err)
		return errorResponse("internal", err.Error())
	}

	s.mu.Lock()
	s.cache[cacheKey] = cacheEntry{secrets: secrets, fetchedAt: time.Now()}
	s.mu.Unlock()

	log.Printf("served %d secrets for %s/%s (decrypted locally)", len(secrets), req.ProjectId, req.Environment)
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
	s.cache = map[string]cacheEntry{}
}

// handleFlush drops every cached bundle so the next SDK request refetches from the
// platform - how a user picks up an admin's secret fix without waiting out the TTL.
func (s *Server) handleFlush() map[string]any {
	s.mu.Lock()
	cleared := len(s.cache)
	s.cache = map[string]cacheEntry{}
	s.mu.Unlock()

	log.Printf("secrets cache flushed (%d bundle(s) dropped)", cleared)
	return map[string]any{"ok": true, "cleared": cleared}
}

func (s *Server) handleStatus() map[string]any {
	token, err := s.token()
	if err != nil {
		return map[string]any{"ok": true, "authenticated": false, "daemonVersion": Version}
	}

	me, err := s.client.Me(token)
	if err != nil {
		return map[string]any{"ok": true, "authenticated": false, "daemonVersion": Version}
	}

	return map[string]any{
		"ok": true, "authenticated": true, "daemonVersion": Version,
		"email": me.Email, "organization": me.Organization,
	}
}

// token returns a live access token, refreshing through the stored (rotating)
// refresh token when the current one is close to expiry.
func (s *Server) token() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.accessToken != "" && time.Until(s.tokenExpiry) > time.Minute {
		return s.accessToken, nil
	}

	session, err := authstore.LoadSession()
	if err != nil {
		return "", err
	}

	tokens, err := s.client.Refresh(session.RefreshToken)
	if err != nil {
		return "", err
	}
	// The refresh token rotates on every use; the device keys must survive it.
	session.RefreshToken = tokens.RefreshToken
	if err := authstore.SaveSession(session); err != nil {
		return "", err
	}

	s.accessToken = tokens.AccessToken
	s.tokenExpiry = time.Now().Add(time.Duration(tokens.ExpiresInSeconds) * time.Second)
	return s.accessToken, nil
}

func asAPIError(err error, target **api.APIError) bool {
	e, ok := err.(*api.APIError)
	if ok {
		*target = e
	}
	return ok
}
