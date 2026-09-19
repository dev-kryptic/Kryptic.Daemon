package provider

import (
	"context"

	go_ora "github.com/sijms/go-ora/v2"
)

func newOracle() Provider {
	return statementProvider{run: func(ctx context.Context, cfg ProviderConfig, script string) error {
		return runSQL(ctx, "oracle", oracleDSN(cfg), script)
	}}
}

func oracleDSN(cfg ProviderConfig) string {
	port := cfg.Port
	if port == 0 {
		port = 1521
	}
	opts := map[string]string{}
	if cfg.SSL {
		opts["SSL"] = "enable"
		if cfg.SSLVerify != nil && !*cfg.SSLVerify {
			opts["SSL Verify"] = "false"
		}
	}
	return go_ora.BuildUrl(cfg.Host, port, cfg.Database, cfg.Username, cfg.Password, opts)
}
