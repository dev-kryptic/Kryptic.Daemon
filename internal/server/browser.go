package server

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/dev-kryptic/daemon/internal/applog"
	"github.com/dev-kryptic/daemon/internal/dynsecret"
	"github.com/dev-kryptic/daemon/internal/orgkey"
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
	writeBrowserCORS(w, r, s.client.BaseURL)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := s.handleStatus()
	writeJSON(w, http.StatusOK, struct {
		OK            bool   `json:"ok"`
		Authenticated bool   `json:"authenticated"`
		OrgKeyGranted bool   `json:"orgKeyGranted"`
		DaemonVersion string `json:"daemonVersion"`
	}{
		OK:            true,
		Authenticated: status.Authenticated,
		OrgKeyGranted: status.OrgKeyGranted,
		DaemonVersion: Version,
	})
}

func (s *Server) handleBrowserLease(w http.ResponseWriter, r *http.Request) {
	writeBrowserCORS(w, r, s.client.BaseURL)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !browserOriginAllowed(r.Header.Get("Origin"), s.client.BaseURL) {
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
		writeJSON(w, http.StatusUnauthorized, struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}{Error: "daemon is not signed in"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	if err := dynsecret.MintLease(ctx, s.client, token, leaseId, orgkey.Device); err != nil {
		writeJSON(w, http.StatusBadRequest, struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, struct {
		OK      bool   `json:"ok"`
		LeaseId string `json:"leaseId"`
	}{OK: true, LeaseId: leaseId})
}

// writeBrowserCORS reflects the Origin only when it is on the dashboard
// allowlist. Random websites cannot read /status (daemon presence, version,
// signed-in state) or the mint response from a victim's browser.
func writeBrowserCORS(w http.ResponseWriter, r *http.Request, baseURL string) {
	if origin := r.Header.Get("Origin"); browserOriginAllowed(origin, baseURL) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Private-Network", "true")
	w.Header().Set("Vary", "Origin")
}

// browserOriginAllowed pins the hosted dashboard (https://app.kryptic.dev),
// loopback hosts for local development, the dashboard implied by the
// self-hosted Daemon BFF the daemon is signed into, and exact-match extras
// from KRYPTIC_DASHBOARD_ORIGINS. No wildcard subdomains and no plain http
// off the loopback: a compromised marketing subdomain or an http MITM page
// must not be able to drive mints.
func browserOriginAllowed(origin, baseURL string) bool {
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
	for _, extra := range strings.Split(os.Getenv("KRYPTIC_DASHBOARD_ORIGINS"), ",") {
		if strings.EqualFold(strings.TrimSpace(extra), origin) {
			return true
		}
	}
	if parsed.Scheme == "https" && host == "app.kryptic.dev" {
		return true
	}
	return selfHostedDashboard(parsed, baseURL)
}

// selfHostedDashboard trusts the dashboard implied by the Daemon BFF URL the
// daemon signed into: the BFF host itself, and its parent when the BFF sits
// on the reference kit's daemon. subdomain (daemon.kryptic.example.com next
// to the dashboard on kryptic.example.com). Signing in is the trust anchor;
// no extra env is needed on a standard self-hosted install. Hosted kryptic.dev
// is excluded: its dashboard pin is explicit and the apex must never qualify.
func selfHostedDashboard(origin *url.URL, baseURL string) bool {
	base, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	baseHost := strings.ToLower(base.Hostname())
	if baseHost == "" || baseHost == "kryptic.dev" || strings.HasSuffix(baseHost, ".kryptic.dev") {
		return false
	}
	if origin.Scheme != "https" && origin.Scheme != base.Scheme {
		return false
	}
	host := strings.ToLower(origin.Hostname())
	return host == baseHost || "daemon."+host == baseHost
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
