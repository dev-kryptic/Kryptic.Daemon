package provider

import (
	"context"
	"encoding/json"
	"fmt"
)

type Provider interface {
	Mint(ctx context.Context, work WorkContext) (Minted, error)
	Renew(ctx context.Context, work WorkContext) error
	Revoke(ctx context.Context, work WorkContext) error
}

func Lookup(kind int) (Provider, error) {
	switch kind {
	case ProviderPostgres:
		return newPostgres(), nil
	case ProviderMySQL:
		return newMySQL(), nil
	case ProviderOracle:
		return newOracle(), nil
	case ProviderCassandra:
		return cassandraProvider{}, nil
	case ProviderRedis:
		return redisProvider{}, nil
	case ProviderAwsIAM:
		return awsProvider{}, nil
	default:
		return nil, fmt.Errorf("unknown dynamic secret provider %d", kind)
	}
}

func ParseProviderConfig(plaintext []byte) (ProviderConfig, error) {
	var cfg ProviderConfig
	if err := json.Unmarshal(plaintext, &cfg); err != nil {
		return cfg, fmt.Errorf("provider config is not JSON: %w", err)
	}
	return cfg, nil
}

type unavailableProvider struct {
	name   string
	reason string
}

func (p unavailableProvider) Mint(context.Context, WorkContext) (Minted, error) {
	return Minted{}, fmt.Errorf("%s: %s", p.name, p.reason)
}
func (p unavailableProvider) Renew(context.Context, WorkContext) error {
	return fmt.Errorf("%s: %s", p.name, p.reason)
}
func (p unavailableProvider) Revoke(context.Context, WorkContext) error {
	return fmt.Errorf("%s: %s", p.name, p.reason)
}
