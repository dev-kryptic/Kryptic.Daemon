package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

func newPostgres() Provider { return statementProvider{run: runPostgres} }

func runPostgres(ctx context.Context, cfg ProviderConfig, script string) error {
	dsn, cleanup, err := postgresDSN(cfg)
	if err != nil {
		return err
	}
	defer cleanup()
	return runSQL(ctx, "pgx", dsn, script)
}

func postgresDSN(cfg ProviderConfig) (string, func(), error) {
	cleanup := func() {}
	port := cfg.Port
	if port == 0 {
		port = 5432
	}
	// verify-full is the default: "require" alone does not check the server
	// certificate, so the admin credential could be intercepted on path.
	// The explicit sslVerify=false toggle keeps encrypted-but-unverified.
	q := url.Values{}
	switch {
	case !cfg.SSL:
		q.Set("sslmode", "disable")
	case cfg.SSLVerify != nil && !*cfg.SSLVerify:
		q.Set("sslmode", "require")
	default:
		q.Set("sslmode", "verify-full")
		if strings.TrimSpace(cfg.CA) != "" {
			path, caCleanup, err := writeTempCA(cfg.CA)
			if err != nil {
				return "", cleanup, fmt.Errorf("write CA bundle: %w", err)
			}
			cleanup = caCleanup
			q.Set("sslrootcert", path)
		}
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.Username, cfg.Password),
		Host:     fmt.Sprintf("%s:%d", cfg.Host, port),
		Path:     "/" + cfg.Database,
		RawQuery: q.Encode(),
	}
	return u.String(), cleanup, nil
}
