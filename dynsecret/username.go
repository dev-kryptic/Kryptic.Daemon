package dynsecret

import (
	"crypto/rand"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// RenderUsername applies Infisical-style templates. Tokens: randomUsername,
// unixTimestamp, identity.name, dynamicSecret.name, dynamicSecret.type,
// random N. Functions: uppercase, lowercase, truncate, replace.
func RenderUsername(template, identityName, dynamicSecretName, dynamicSecretType string) string {
	source := strings.TrimSpace(template)
	if source == "" {
		source = "{{randomUsername}}"
	}
	randomUsername := RandomString(32)
	unix := strconv.FormatInt(time.Now().Unix(), 10)
	identity := Sanitize(identityName)
	secretName := Sanitize(dynamicSecretName)
	secretType := Sanitize(dynamicSecretType)

	tokens := map[string]string{
		"randomUsername":     randomUsername,
		"unixTimestamp":      unix,
		"identity.name":      identity,
		"dynamicSecret.name": secretName,
		"dynamicSecret.type": secretType,
	}
	rendered := source
	for name, value := range tokens {
		rendered = replaceToken(rendered, name, value)
	}
	rendered = replaceRandomN(rendered)
	rendered = applyFunctions(rendered, tokens)
	if rendered == "" {
		return randomUsername
	}
	return rendered
}

func replaceToken(source, name, value string) string {
	source = strings.ReplaceAll(source, "{{"+name+"}}", value)
	return strings.ReplaceAll(source, "{{ "+name+" }}", value)
}

func replaceRandomN(source string) string {
	for {
		start := strings.Index(source, "{{random ")
		if start < 0 {
			start = strings.Index(source, "{{ random ")
		}
		if start < 0 {
			return source
		}
		end := strings.Index(source[start:], "}}")
		if end < 0 {
			return source
		}
		end += start
		inner := strings.TrimSpace(source[start+2 : end])
		parts := strings.Fields(inner)
		n := 8
		if len(parts) == 2 {
			if parsed, err := strconv.Atoi(parts[1]); err == nil {
				if parsed < 1 {
					parsed = 1
				}
				if parsed > 64 {
					parsed = 64
				}
				n = parsed
			}
		}
		source = source[:start] + RandomString(n) + source[end+2:]
	}
}

func applyFunctions(source string, tokens map[string]string) string {
	source = applyUnary(source, "uppercase", strings.ToUpper, tokens)
	source = applyUnary(source, "lowercase", strings.ToLower, tokens)
	source = applyTruncate(source, tokens)
	source = applyReplace(source, tokens)
	return source
}

func resolveArg(value string, tokens map[string]string) string {
	if tokens != nil {
		if mapped, ok := tokens[value]; ok {
			return mapped
		}
	}
	return value
}

func applyUnary(source, name string, apply func(string) string, tokens map[string]string) string {
	open := "{{" + name + " "
	for {
		start := strings.Index(source, open)
		if start < 0 {
			return source
		}
		end := strings.Index(source[start:], "}}")
		if end < 0 {
			return source
		}
		end += start
		value := resolveArg(strings.TrimSpace(source[start+len(open):end]), tokens)
		source = source[:start] + apply(value) + source[end+2:]
	}
}

func applyTruncate(source string, tokens map[string]string) string {
	const open = "{{truncate "
	for {
		start := strings.Index(source, open)
		if start < 0 {
			return source
		}
		end := strings.Index(source[start:], "}}")
		if end < 0 {
			return source
		}
		end += start
		inner := strings.TrimSpace(source[start+len(open) : end])
		parts := strings.Fields(inner)
		value := ""
		if len(parts) > 0 {
			value = resolveArg(parts[0], tokens)
		}
		n := len(value)
		if len(parts) > 1 {
			if parsed, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
				n = parsed
			}
		}
		if n < 0 {
			n = 0
		}
		if n > len(value) {
			n = len(value)
		}
		source = source[:start] + value[:n] + source[end+2:]
	}
}

func applyReplace(source string, tokens map[string]string) string {
	const open = "{{replace "
	for {
		start := strings.Index(source, open)
		if start < 0 {
			return source
		}
		end := strings.Index(source[start:], "}}")
		if end < 0 {
			return source
		}
		end += start
		args := splitQuoted(strings.TrimSpace(source[start+len(open) : end]))
		value, from, to := "", "", ""
		if len(args) > 0 {
			value = resolveArg(args[0], tokens)
		}
		if len(args) > 1 {
			from = args[1]
		}
		if len(args) > 2 {
			to = args[2]
		}
		source = source[:start] + strings.ReplaceAll(value, from, to) + source[end+2:]
	}
}

func splitQuoted(input string) []string {
	var parts []string
	var current strings.Builder
	quoted := false
	for _, ch := range input {
		if ch == '\'' || ch == '"' {
			quoted = !quoted
			continue
		}
		if !quoted && unicode.IsSpace(ch) {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(ch)
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

func Sanitize(value string) string {
	if strings.TrimSpace(value) == "" {
		return "kryptic"
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "kryptic"
	}
	return b.String()
}

// RandomString draws uniformly from the 62-character alphabet. Bytes >= 248
// are rejected instead of reduced modulo 62, so no character is more likely
// than another. It panics if the OS entropy source fails: minting a lease
// password from anything weaker must never happen silently.
func RandomString(length int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const limit = byte(256 - 256%len(alphabet)) // 248
	if length < 1 {
		length = 1
	}
	out := make([]byte, 0, length)
	buf := make([]byte, 64)
	for len(out) < length {
		if _, err := rand.Read(buf); err != nil {
			panic("kryptic: crypto/rand is unavailable: " + err.Error())
		}
		for _, b := range buf {
			if b >= limit {
				continue
			}
			out = append(out, alphabet[int(b)%len(alphabet)])
			if len(out) == length {
				break
			}
		}
	}
	return string(out)
}
