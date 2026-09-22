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
	"encoding/json"
	"log"
	"net"
	"sync"
	"time"

	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/applog"
	"github.com/dev-kryptic/daemon/internal/ipc"
	"github.com/dev-kryptic/daemon/internal/orgkey"
)

const (
	protocolVersion = 1
	cacheTTL        = 5 * time.Minute
)

// Version is the daemon version reported to the platform and compared by
// `kryptic update`. Release builds override it with:
// -ldflags "-X github.com/dev-kryptic/daemon/internal/server.Version=x.y.z"
var Version = "1.0.0"

type cacheEntry struct {
	secrets   []orgkey.Pair
	hints     []api.DynamicHint
	orgKey    []byte
	leaseIDs  []string
	fetchedAt time.Time
}

type Server struct {
	client *api.Client

	mu            sync.Mutex
	accessToken   string
	tokenExpiry   time.Time
	cache         map[string]cacheEntry
	lastMe        *api.Me
	lastStatusLog time.Time
}

func New(client *api.Client) *Server {
	return &Server{client: client, cache: map[string]cacheEntry{}}
}

func (s *Server) Run() error {
	listener, err := ipc.Listen()
	if err != nil {
		return err
	}
	defer listener.Close()

	log.Printf("kryptic daemon %s listening on %s (api: %s)", Version, ipc.Endpoint(), s.client.BaseURL)
	applog.Event("daemon", "start", "version="+Version)
	applog.Event("daemon", "listen")

	go s.serveBrowser()
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
	ProfileId   string `json:"profileId"`
	API         string `json:"api"`
}

func (s *Server) handle(connection net.Conn) {
	defer connection.Close()

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
		s.reply(connection, fail("internal", "invalid request"))
		return
	}
	if req.V != protocolVersion {
		s.reply(connection, fail("unsupported_version", "this daemon speaks protocol v1"))
		return
	}

	s.applyActiveAPI()

	switch req.Type {
	case "secrets":
		s.reply(connection, s.handleSecrets(req))
	case "status":
		s.reply(connection, s.handleStatus())
	case "flush":
		s.reply(connection, s.handleFlush())
	case "reset-auth":
		s.reply(connection, s.handleResetAuth())
	case "switch-profile":
		s.reply(connection, s.handleSwitchProfile(req))
	case "set-api":
		s.reply(connection, s.handleSetAPI(req))
	case "delete-profile":
		s.reply(connection, s.handleDeleteProfile(req))
	default:
		s.reply(connection, fail("internal", "unknown request type"))
	}
}
