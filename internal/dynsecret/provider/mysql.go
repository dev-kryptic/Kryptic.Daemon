package provider

import (
	"context"
	"fmt"
)

func newMySQL() Provider {
	return statementProvider{run: func(ctx context.Context, cfg ProviderConfig, script string) error {
		return runSQL(ctx, "mysql", mysqlDSN(cfg), script)
	}}
}

func mysqlDSN(cfg ProviderConfig) string {
	port := cfg.Port
	if port == 0 {
		port = 3306
	}
	tls := "false"
	if cfg.SSL {
		tls = "true"
		if cfg.SSLVerify != nil && !*cfg.SSLVerify {
			tls = "skip-verify"
		}
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?tls=%s&multiStatements=true&parseTime=true",
		cfg.Username, cfg.Password, cfg.Host, port, cfg.Database, tls)
}
