package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/dev-kryptic/Kryptic.Encryption.Go/sealedbox"
	"github.com/dev-kryptic/daemon/internal/applog"
	"github.com/dev-kryptic/daemon/internal/authstore"
	"github.com/dev-kryptic/daemon/internal/dynsecret"
)

// BrowserPort is the loopback HTTP port the dashboard probes before Generate lease.
const BrowserPort = "5278"

func (s *Server) serveBrowser() {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", s.handleBrowserStatus)
	mux.HandleFunc("/dynamic-secrets/leases/", s.handleBrowserLease)

	addr := net.JoinHostPort("127.0.0.1", BrowserPort)
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}
	log.Printf("dashboard loopback on http://%s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("dashboard loopback: %v", err)
		applog.Error("daemon", "browser.listen", err)
	}
}

func (s *Server) handleBrowserStatus(w http.ResponseWriter, r *http.Request) {
	writeBrowserCORS(w, r, true)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := s.handleStatus()
	authenticated, _ := status["authenticated"].(bool)
	orgKeyGranted, _ := status["orgKeyGranted"].(bool)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"authenticated":  authenticated,
		"orgKeyGranted":  orgKeyGranted,
		"daemonVersion":  Version,
	})
}

func (s *Server) handleBrowserLease(w http.ResponseWriter, r *http.Request) {
	writeBrowserCORS(w, r, false)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !browserOriginAllowed(r.Header.Get("Origin")) {
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return
	}

	const prefix = "/dynamic-secrets/leases/"
	const suffix = "/mint"
	if !strings.HasPrefix(r.URL.Path, prefix) || !strings.HasSuffix(r.URL.Path, suffix) {
		http.Error(w, "unknown lease", http.StatusNotFound)
		return
	}
	leaseId := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), suffix)
	if leaseId == "" {
		http.Error(w, "unknown lease", http.StatusNotFound)
		return
	}

	token, err := s.token()
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "daemon is not signed in"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	if err := dynsecret.MintLease(ctx, s.client, token, leaseId, unwrapDeviceOrgKey); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "leaseId": leaseId})
}

func unwrapDeviceOrgKey(_, wrapped string) ([]byte, error) {
	session, err := authstore.LoadSession()
	if err != nil {
		return nil, err
	}
	enc := base64.RawURLEncoding
	private, err := enc.DecodeString(session.DevicePrivateKey)
	if err != nil {
		return nil, err
	}
	public, err := enc.DecodeString(session.DevicePublicKey)
	if err != nil {
		return nil, err
	}
	sealed, err := sealedbox.Parse(wrapped)
	if err != nil {
		return nil, err
	}
	return sealedbox.Open(sealedbox.KeyPair{Public: public, Private: private}, sealed)
}

func writeBrowserCORS(w http.ResponseWriter, r *http.Request, allowAny bool) {
	origin := r.Header.Get("Origin")
	if allowAny {
		if origin == "" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
	} else if browserOriginAllowed(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Private-Network", "true")
	w.Header().Set("Vary", "Origin")
}

func browserOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	if host == "app.kryptic.dev" || strings.HasSuffix(host, ".kryptic.dev") {
		return true
	}
	for _, extra := range strings.Split(os.Getenv("KRYPTIC_DASHBOARD_ORIGINS"), ",") {
		if strings.EqualFold(strings.TrimSpace(extra), origin) {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, body map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
