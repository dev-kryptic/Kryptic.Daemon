package dynsecret

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func runSQL(ctx context.Context, driver, dsn, script string) error {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return err
	}
	for _, stmt := range splitStatements(script) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("%s: %w", truncate(stmt, 80), err)
		}
	}
	return nil
}

func splitStatements(script string) []string {
	var out []string
	for _, part := range strings.Split(script, ";") {
		stmt := strings.TrimSpace(part)
		if stmt != "" {
			out = append(out, stmt)
		}
	}
	return out
}

func truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

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

func mysqlDSN(cfg ProviderConfig) string {
	port := cfg.Port
	if port == 0 {
		port = 3306
	}
	tls := "false"
	if cfg.SSL {
		tls = "true" // verifies the server certificate
		if cfg.SSLVerify != nil && !*cfg.SSLVerify {
			tls = "skip-verify"
		}
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?tls=%s&multiStatements=true&parseTime=true",
		cfg.Username, cfg.Password, cfg.Host, port, cfg.Database, tls)
}

// writeTempCA stages a PEM CA bundle from the provider config so drivers that
// want a file path can verify against it. The caller must run cleanup, which
// is a no-op when no CA is configured. A write failure returns an error so
// verify-full cannot silently fall back to the system store.
func writeTempCA(ca string) (path string, cleanup func(), err error) {
	cleanup = func() {}
	if strings.TrimSpace(ca) == "" {
		return "", cleanup, nil
	}
	f, err := os.CreateTemp("", "kryptic-ca-*.pem")
	if err != nil {
		return "", cleanup, err
	}
	name := f.Name()
	cleanup = func() { _ = os.Remove(name) }
	if _, err := f.WriteString(ca); err != nil {
		f.Close()
		cleanup()
		return "", func() {}, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return name, cleanup, nil
}

type postgresProvider struct{}

func (postgresProvider) Mint(ctx context.Context, work WorkContext) (Minted, error) {
	script := RenderStatements(work.CreationStatement, StatementVars(work))
	if err := runPostgres(ctx, work.Config, script); err != nil {
		return Minted{}, err
	}
	return Minted{Username: work.Username, Password: work.Password}, nil
}

func (postgresProvider) Renew(ctx context.Context, work WorkContext) error {
	if strings.TrimSpace(work.RenewStatement) == "" {
		return nil
	}
	return runPostgres(ctx, work.Config, RenderStatements(work.RenewStatement, StatementVars(work)))
}

func (postgresProvider) Revoke(ctx context.Context, work WorkContext) error {
	return runPostgres(ctx, work.Config, RenderStatements(work.RevocationStatement, StatementVars(work)))
}

type mysqlProvider struct{}

func (mysqlProvider) Mint(ctx context.Context, work WorkContext) (Minted, error) {
	if err := runSQL(ctx, "mysql", mysqlDSN(work.Config), RenderStatements(work.CreationStatement, StatementVars(work))); err != nil {
		return Minted{}, err
	}
	return Minted{Username: work.Username, Password: work.Password}, nil
}

func (mysqlProvider) Renew(ctx context.Context, work WorkContext) error {
	if strings.TrimSpace(work.RenewStatement) == "" {
		return nil
	}
	return runSQL(ctx, "mysql", mysqlDSN(work.Config), RenderStatements(work.RenewStatement, StatementVars(work)))
}

func (mysqlProvider) Revoke(ctx context.Context, work WorkContext) error {
	return runSQL(ctx, "mysql", mysqlDSN(work.Config), RenderStatements(work.RevocationStatement, StatementVars(work)))
}
