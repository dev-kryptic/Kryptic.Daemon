package provider

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type scriptRunner func(context.Context, ProviderConfig, string) error

// statementProvider is the shared Mint/Renew/Revoke loop for drivers that
// run a rendered SQL script (Postgres, MySQL, Oracle).
type statementProvider struct {
	run scriptRunner
}

func (p statementProvider) Mint(ctx context.Context, work WorkContext) (Minted, error) {
	if err := p.run(ctx, work.Config, RenderStatements(work.CreationStatement, StatementVars(work))); err != nil {
		return Minted{}, err
	}
	return Minted{Username: work.Username, Password: work.Password}, nil
}

func (p statementProvider) Renew(ctx context.Context, work WorkContext) error {
	if strings.TrimSpace(work.RenewStatement) == "" {
		return nil
	}
	return p.run(ctx, work.Config, RenderStatements(work.RenewStatement, StatementVars(work)))
}

func (p statementProvider) Revoke(ctx context.Context, work WorkContext) error {
	return p.run(ctx, work.Config, RenderStatements(work.RevocationStatement, StatementVars(work)))
}

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
