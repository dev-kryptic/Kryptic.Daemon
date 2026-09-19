package dynsecret

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	ProviderPostgres  = 0
	ProviderMySQL     = 1
	ProviderOracle    = 2
	ProviderCassandra = 3
	ProviderRedis     = 4
	ProviderAwsIAM    = 5

	ActionCreate = 0
	ActionRenew  = 1
	ActionRevoke = 2
)

type ProviderConfig struct {
	Host              string `json:"host"`
	Port              int    `json:"port"`
	Username          string `json:"username"`
	Password          string `json:"password"`
	Database          string `json:"database"`
	SSL               bool   `json:"ssl"`
	CA                string `json:"ca"`
	SSLVerify         *bool  `json:"sslVerify"`
	Hosts             string `json:"hosts"`
	LocalDc           string `json:"localDc"`
	Keyspace          string `json:"keyspace"`
	TLS               bool   `json:"tls"`
	AuthMethod        string `json:"authMethod"`
	CredentialType    string `json:"credentialType"`
	AccessKeyId       string `json:"accessKeyId"`
	SecretAccessKey   string `json:"secretAccessKey"`
	RoleArn           string `json:"roleArn"`
	Region            string `json:"region"`
	DurationSeconds   int    `json:"durationSeconds"`
	PolicyDocument    string `json:"policyDocument"`
	PermissionBoundary string `json:"permissionBoundary"`
	IamUserPath       string `json:"iamUserPath"`
}

type Minted struct {
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	SessionToken   string `json:"sessionToken,omitempty"`
	AccessKeyId    string `json:"accessKeyId,omitempty"`
	URL            string `json:"url,omitempty"`
}

type WorkContext struct {
	Config            ProviderConfig
	Username          string
	Password          string
	CreationStatement string
	RevocationStatement string
	RenewStatement    string
	URLTemplate       string
	ExpiresAt         time.Time
	EntityId          string
}

type Provider interface {
	Mint(ctx context.Context, work WorkContext) (Minted, error)
	Renew(ctx context.Context, work WorkContext) error
	Revoke(ctx context.Context, work WorkContext) error
}

func Lookup(provider int) (Provider, error) {
	switch provider {
	case ProviderPostgres:
		return postgresProvider{}, nil
	case ProviderMySQL:
		return mysqlProvider{}, nil
	case ProviderOracle:
		return newOracle(), nil
	case ProviderCassandra:
		return cassandraProvider{}, nil
	case ProviderRedis:
		return redisProvider{}, nil
	case ProviderAwsIAM:
		return awsProvider{}, nil
	default:
		return nil, fmt.Errorf("unknown dynamic secret provider %d", provider)
	}
}

func ParseProviderConfig(plaintext []byte) (ProviderConfig, error) {
	var cfg ProviderConfig
	if err := json.Unmarshal(plaintext, &cfg); err != nil {
		return cfg, fmt.Errorf("provider config is not JSON: %w", err)
	}
	return cfg, nil
}

func RenderStatements(template string, vars map[string]string) string {
	out := template
	for key, value := range vars {
		out = strings.ReplaceAll(out, "{{"+key+"}}", value)
		out = strings.ReplaceAll(out, "{{ "+key+" }}", value)
	}
	return out
}

func StatementVars(work WorkContext) map[string]string {
	return map[string]string{
		"username":   work.Username,
		"password":   work.Password,
		"expiration": work.ExpiresAt.UTC().Format("2006-01-02 15:04:05+00"),
		"database":   firstNonEmpty(work.Config.Database, work.Config.Keyspace),
		"keyspace":   firstNonEmpty(work.Config.Keyspace, work.Config.Database),
	}
}

func RenderURL(template string, minted Minted, cfg ProviderConfig, expires time.Time) string {
	if strings.TrimSpace(template) == "" {
		return ""
	}
	return RenderStatements(template, map[string]string{
		"username":     minted.Username,
		"password":     minted.Password,
		"host":         firstNonEmpty(cfg.Host, firstHost(cfg.Hosts)),
		"port":         fmt.Sprintf("%d", cfg.Port),
		"database":     firstNonEmpty(cfg.Database, cfg.Keyspace),
		"accessKeyId":  minted.AccessKeyId,
		"sessionToken": minted.SessionToken,
		"expiration":   expires.UTC().Format(time.RFC3339),
	})
}

func firstHost(hosts string) string {
	parts := strings.Split(hosts, ",")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
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
