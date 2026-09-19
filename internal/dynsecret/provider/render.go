package provider

import (
	"fmt"
	"strings"
	"time"
)

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
