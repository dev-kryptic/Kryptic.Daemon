package dynsecret

import (
	"context"
	"strings"

	"github.com/gocql/gocql"
)

type cassandraProvider struct{}

func cassandraSession(cfg ProviderConfig) (*gocql.Session, func(), error) {
	cleanup := func() {}
	hosts := strings.Split(cfg.Hosts, ",")
	if len(hosts) == 1 && strings.TrimSpace(hosts[0]) == "" && cfg.Host != "" {
		hosts = []string{cfg.Host}
	}
	for i := range hosts {
		hosts[i] = strings.TrimSpace(hosts[i])
	}
	cluster := gocql.NewCluster(hosts...)
	if cfg.Port > 0 {
		cluster.Port = cfg.Port
	}
	if cfg.LocalDc != "" {
		cluster.PoolConfig.HostSelectionPolicy = gocql.DCAwareRoundRobinPolicy(cfg.LocalDc)
	}
	if cfg.Username != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{Username: cfg.Username, Password: cfg.Password}
	}
	if cfg.Keyspace != "" {
		cluster.Keyspace = cfg.Keyspace
	} else if cfg.Database != "" {
		cluster.Keyspace = cfg.Database
	}
	if cfg.TLS || cfg.SSL {
		opts := &gocql.SslOptions{
			EnableHostVerification: cfg.SSLVerify == nil || *cfg.SSLVerify,
		}
		if strings.TrimSpace(cfg.CA) != "" {
			path, caCleanup, err := writeTempCA(cfg.CA)
			if err != nil {
				return nil, cleanup, err
			}
			cleanup = caCleanup
			opts.CaPath = path
		}
		cluster.SslOpts = opts
	}
	session, err := cluster.CreateSession()
	if err != nil {
		cleanup()
		return nil, func() {}, err
	}
	return session, cleanup, nil
}

func (cassandraProvider) Mint(ctx context.Context, work WorkContext) (Minted, error) {
	session, cleanup, err := cassandraSession(work.Config)
	if err != nil {
		return Minted{}, err
	}
	defer cleanup()
	defer session.Close()
	if err := cqlExec(ctx, session, RenderStatements(work.CreationStatement, StatementVars(work))); err != nil {
		return Minted{}, err
	}
	return Minted{Username: work.Username, Password: work.Password}, nil
}

func (cassandraProvider) Renew(context.Context, WorkContext) error { return nil }

func (cassandraProvider) Revoke(ctx context.Context, work WorkContext) error {
	session, cleanup, err := cassandraSession(work.Config)
	if err != nil {
		return err
	}
	defer cleanup()
	defer session.Close()
	return cqlExec(ctx, session, RenderStatements(work.RevocationStatement, StatementVars(work)))
}

func cqlExec(ctx context.Context, session *gocql.Session, script string) error {
	for _, stmt := range splitStatements(script) {
		if err := session.Query(stmt).WithContext(ctx).Exec(); err != nil {
			return err
		}
	}
	return nil
}
