// Package applog writes a size-capped diagnostics file for support.
// Lines record app function only: start, auth refresh, HTTP status, updates.
// Secrets, tokens, emails, display names, and device names never belong here.
package applog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dev-kryptic/daemon/internal/config"
)

const (
	FileName   = "kryptic.krypticlog"
	backupName = "kryptic.krypticlog.1"
)

// maxFileBytes caps the current log. One rotated backup is kept, so the
// folder stays around 4 MiB. Tests lower this to exercise rotation.
var maxFileBytes int64 = 2 << 20

var (
	mu sync.Mutex

	emailPattern  = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	bearerPattern = regexp.MustCompile(`(?i)bearer\s+\S+`)
	tokenPattern  = regexp.MustCompile(`(?i)\b(refreshToken|accessToken|clientSecret|deviceCode|userCode|authorization)\b([:=]\s*)\S+`)
	b64Pattern    = regexp.MustCompile(`\b[A-Za-z0-9_-]{32,}\b`)
)

// Dir is …/kryptic/logs under the per-user config directory.
func Dir() (string, error) {
	base, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "logs"), nil
}

// Path is the current diagnostics file.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

// Event appends one diagnostics line. fields are key=value; values are sanitized.
func Event(source, event string, fields ...string) {
	write(source, event, fields)
}

// Error is Event plus a sanitized error class (never the raw secret-bearing string).
func Error(source, event string, err error, fields ...string) {
	if err != nil {
		fields = append(fields, "class="+errorClass(err))
	}
	write(source, event, fields)
}

func write(source, event string, fields []string) {
	source = sanitizeToken(source)
	event = sanitizeToken(event)
	if source == "" {
		source = "kryptic"
	}
	if event == "" {
		event = "event"
	}

	parts := make([]string, 0, 2+len(fields))
	parts = append(parts, time.Now().UTC().Format(time.RFC3339), source, event)
	for _, field := range fields {
		if field == "" {
			continue
		}
		parts = append(parts, Sanitize(field))
	}
	line := strings.Join(parts, " ") + "\n"

	mu.Lock()
	defer mu.Unlock()
	_ = appendLine(line)
}

func appendLine(line string) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	if info, err := os.Stat(path); err == nil && info.Size()+int64(len(line)) > maxFileBytes {
		backup := filepath.Join(filepath.Dir(path), backupName)
		_ = os.Remove(backup)
		if err := os.Rename(path, backup); err != nil {
			_ = os.Truncate(path, 0)
		}
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(line)
	return err
}

// Sanitize strips emails, bearer tokens, and long opaque strings so a field
// can be written even if a caller passed an error message.
func Sanitize(raw string) string {
	raw = strings.ReplaceAll(raw, "\n", " ")
	raw = strings.ReplaceAll(raw, "\r", " ")
	raw = emailPattern.ReplaceAllString(raw, "[email]")
	raw = bearerPattern.ReplaceAllString(raw, "bearer [token]")
	raw = tokenPattern.ReplaceAllString(raw, "${1}${2}[token]")
	raw = b64Pattern.ReplaceAllString(raw, "[redacted]")
	return strings.TrimSpace(raw)
}

func sanitizeToken(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, " ", "_")
	return Sanitize(raw)
}

func errorClass(err error) string {
	if err == nil {
		return "ok"
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "not logged in"):
		return "not_logged_in"
	case strings.Contains(text, "invalid refresh"):
		return "invalid_refresh"
	case strings.Contains(text, "expired by organization"):
		return "session_expired"
	case strings.Contains(text, "deactivated"):
		return "account_deactivated"
	case strings.Contains(text, "timeout") || strings.Contains(text, "deadline"):
		return "timeout"
	case strings.Contains(text, "connection refused") || strings.Contains(text, "no such host"):
		return "network"
	case strings.Contains(text, "429") || strings.Contains(text, "too many"):
		return "rate_limited"
	case strings.Contains(text, "401") || strings.Contains(text, "unauthorized"):
		return "unauthorized"
	case strings.Contains(text, "403") || strings.Contains(text, "forbidden"):
		return "forbidden"
	case strings.Contains(text, "500") || strings.Contains(text, "502") || strings.Contains(text, "503"):
		return "server"
	default:
		return "error"
	}
}

// StatusLine is what `kryptic logs` prints: path plus a one-line hint.
func StatusLine() (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s\nDiagnostics only (no secrets, tokens, or names). Send this file to support.", path), nil
}
