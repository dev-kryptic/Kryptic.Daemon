package dynsecret

import (
	"context"
	"strings"

	go_ora "github.com/sijms/go-ora/v2"
)

type oracleProvider struct{}

func newOracle() Provider { return oracleProvider{} }

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

func (oracleProvider) Mint(ctx context.Context, work WorkContext) (Minted, error) {
	if err := runSQL(ctx, "oracle", oracleDSN(work.Config), RenderStatements(work.CreationStatement, StatementVars(work))); err != nil {
		return Minted{}, err
	}
	return Minted{Username: work.Username, Password: work.Password}, nil
}

func (oracleProvider) Renew(ctx context.Context, work WorkContext) error {
	if strings.TrimSpace(work.RenewStatement) == "" {
		return nil
	}
	return runSQL(ctx, "oracle", oracleDSN(work.Config), RenderStatements(work.RenewStatement, StatementVars(work)))
}

func (oracleProvider) Revoke(ctx context.Context, work WorkContext) error {
	return runSQL(ctx, "oracle", oracleDSN(work.Config), RenderStatements(work.RevocationStatement, StatementVars(work)))
}
