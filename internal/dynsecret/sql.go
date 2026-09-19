package dynsecret

import (
	"context"
	"database/sql"
	"fmt"
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

func postgresDSN(cfg ProviderConfig) string {
	port := cfg.Port
	if port == 0 {
		port = 5432
	}
	ssl := "require"
	if !cfg.SSL {
		ssl = "disable"
	} else if cfg.SSLVerify != nil && !*cfg.SSLVerify {
		ssl = "require"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Username, cfg.Password, cfg.Host, port, cfg.Database, ssl)
}

func mysqlDSN(cfg ProviderConfig) string {
	port := cfg.Port
	if port == 0 {
		port = 3306
	}
	tls := "false"
	if cfg.SSL {
		tls = "true"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?tls=%s&multiStatements=true&parseTime=true",
		cfg.Username, cfg.Password, cfg.Host, port, cfg.Database, tls)
}

type postgresProvider struct{}

func (postgresProvider) Mint(ctx context.Context, work WorkContext) (Minted, error) {
	script := RenderStatements(work.CreationStatement, StatementVars(work))
	if err := runSQL(ctx, "pgx", postgresDSN(work.Config), script); err != nil {
		return Minted{}, err
	}
	return Minted{Username: work.Username, Password: work.Password}, nil
}

func (postgresProvider) Renew(ctx context.Context, work WorkContext) error {
	if strings.TrimSpace(work.RenewStatement) == "" {
		return nil
	}
	return runSQL(ctx, "pgx", postgresDSN(work.Config), RenderStatements(work.RenewStatement, StatementVars(work)))
}

func (postgresProvider) Revoke(ctx context.Context, work WorkContext) error {
	return runSQL(ctx, "pgx", postgresDSN(work.Config), RenderStatements(work.RevocationStatement, StatementVars(work)))
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
