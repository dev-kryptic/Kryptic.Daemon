package dynsecret

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

type redisProvider struct{}

func redisClient(cfg ProviderConfig) *redis.Client {
	port := cfg.Port
	if port == 0 {
		port = 6379
	}
	opts := &redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, port),
		Username: cfg.Username,
		Password: cfg.Password,
	}
	if cfg.TLS || cfg.SSL {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: cfg.SSLVerify != nil && !*cfg.SSLVerify}
	}
	return redis.NewClient(opts)
}

func (redisProvider) Mint(ctx context.Context, work WorkContext) (Minted, error) {
	client := redisClient(work.Config)
	defer client.Close()
	cmd := RenderStatements(work.CreationStatement, StatementVars(work))
	if err := redisDo(ctx, client, cmd); err != nil {
		return Minted{}, err
	}
	return Minted{Username: work.Username, Password: work.Password}, nil
}

func (redisProvider) Renew(context.Context, WorkContext) error { return nil }

func (redisProvider) Revoke(ctx context.Context, work WorkContext) error {
	client := redisClient(work.Config)
	defer client.Close()
	return redisDo(ctx, client, RenderStatements(work.RevocationStatement, StatementVars(work)))
}

func redisDo(ctx context.Context, client *redis.Client, script string) error {
	for _, line := range strings.Split(script, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		args := strings.Fields(line)
		raw := make([]any, len(args))
		for i, a := range args {
			raw[i] = a
		}
		if err := client.Do(ctx, raw...).Err(); err != nil {
			return fmt.Errorf("%s: %w", line, err)
		}
	}
	return nil
}
