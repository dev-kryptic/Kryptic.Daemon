// Package scan implements `kryptic scan`: local secret detection using the
// gitleaks default ruleset (MIT). gitleaks is itself a Go program, so its
// regexes run natively on Go's regexp engine - no translation needed.
//
// The embedded config is machine-generated with a fixed shape, so a focused
// parser covers it fully: the global [allowlist], [[rules]] entries
// (id/description/regex/path/entropy/keywords) and per-rule [[rules.allowlists]]
// (regexes/paths/stopwords, with optional regexTarget/condition).
package scan

import (
	_ "embed"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

//go:embed gitleaks.toml
var gitleaksConfig string

// Rule is one detection rule from the config.
type Rule struct {
	ID          string
	Description string
	Regex       *regexp.Regexp
	Path        *regexp.Regexp // nil unless the rule is path-scoped
	Entropy     float64        // 0 = no entropy requirement
	Keywords    []string       // lowercase pre-filter; empty = always run the regex
	Allowlists  []Allowlist
}

// Allowlist suppresses matches, either globally (paths/regexes) or per rule.
type Allowlist struct {
	Paths       []*regexp.Regexp
	Regexes     []*regexp.Regexp
	Stopwords   []string
	RegexTarget string // "" (secret), "match" or "line"
	MatchAll    bool   // condition = "AND": every populated field must match
}

// Config is the parsed ruleset.
type Config struct {
	Global Allowlist
	Rules  []Rule
}

// Load parses the embedded gitleaks config. It is strict: an unparseable rule
// is a bug in the embedded file, not a runtime condition, so Load fails loudly.
func Load() (*Config, error) {
	return parse(gitleaksConfig)
}

// parse walks the TOML line by line. The generated file only ever contains
// `key = value` pairs, `key = [ … ]` arrays (possibly multiline with '''…'''
// literals) and the three section kinds handled below.
func parse(input string) (*Config, error) {
	config := &Config{}

	type section int
	const (
		none section = iota
		globalAllowlist
		rule
		ruleAllowlist
	)
	current := none

	var pendingKey string
	var pendingValues []string
	inArray := false

	flush := func(key string, values []string) error {
		switch current {
		case globalAllowlist:
			return applyAllowlistKey(&config.Global, key, values)
		case rule:
			return applyRuleKey(&config.Rules[len(config.Rules)-1], key, values)
		case ruleAllowlist:
			currentRule := &config.Rules[len(config.Rules)-1]
			return applyAllowlistKey(&currentRule.Allowlists[len(currentRule.Allowlists)-1], key, values)
		}
		return nil
	}

	for lineNumber, rawLine := range strings.Split(input, "\n") {
		line := strings.TrimSpace(rawLine)

		if inArray {
			if line == "]" {
				inArray = false
				if err := flush(pendingKey, pendingValues); err != nil {
					return nil, fmt.Errorf("line %d: %w", lineNumber+1, err)
				}
				continue
			}
			if value, ok := arrayElement(line); ok {
				pendingValues = append(pendingValues, value)
			}
			continue
		}

		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case line == "[allowlist]":
			current = globalAllowlist
		case line == "[[rules]]":
			config.Rules = append(config.Rules, Rule{})
			current = rule
		case line == "[[rules.allowlists]]":
			currentRule := &config.Rules[len(config.Rules)-1]
			currentRule.Allowlists = append(currentRule.Allowlists, Allowlist{})
			current = ruleAllowlist
		case strings.HasPrefix(line, "["):
			current = none // any other section ([extend], …) is irrelevant here
		default:
			key, rest, found := strings.Cut(line, "=")
			if !found {
				continue
			}
			key = strings.TrimSpace(key)
			rest = strings.TrimSpace(rest)

			if rest == "[" {
				pendingKey, pendingValues, inArray = key, nil, true
				continue
			}
			if strings.HasPrefix(rest, "[") && strings.HasSuffix(rest, "]") {
				if err := flush(key, splitInlineArray(rest)); err != nil {
					return nil, fmt.Errorf("line %d: %w", lineNumber+1, err)
				}
				continue
			}
			if err := flush(key, []string{unquote(rest)}); err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNumber+1, err)
			}
		}
	}

	if len(config.Rules) == 0 {
		return nil, fmt.Errorf("no rules parsed - embedded config is broken")
	}
	return config, nil
}

func applyRuleKey(target *Rule, key string, values []string) error {
	switch key {
	case "id":
		target.ID = values[0]
	case "description":
		target.Description = values[0]
	case "regex":
		compiled, err := regexp.Compile(values[0])
		if err != nil {
			return fmt.Errorf("rule %s: %w", target.ID, err)
		}
		target.Regex = compiled
	case "path":
		compiled, err := regexp.Compile(values[0])
		if err != nil {
			return fmt.Errorf("rule %s path: %w", target.ID, err)
		}
		target.Path = compiled
	case "entropy":
		entropy, err := strconv.ParseFloat(values[0], 64)
		if err != nil {
			return fmt.Errorf("rule %s entropy: %w", target.ID, err)
		}
		target.Entropy = entropy
	case "keywords":
		for _, keyword := range values {
			target.Keywords = append(target.Keywords, strings.ToLower(keyword))
		}
	case "secretGroup":
		// The capture-group hint; the engine already prefers group 1 when present.
	}
	return nil
}

func applyAllowlistKey(target *Allowlist, key string, values []string) error {
	switch key {
	case "paths":
		for _, value := range values {
			compiled, err := regexp.Compile(value)
			if err != nil {
				return fmt.Errorf("allowlist path: %w", err)
			}
			target.Paths = append(target.Paths, compiled)
		}
	case "regexes":
		for _, value := range values {
			compiled, err := regexp.Compile(value)
			if err != nil {
				return fmt.Errorf("allowlist regex: %w", err)
			}
			target.Regexes = append(target.Regexes, compiled)
		}
	case "stopwords":
		for _, value := range values {
			target.Stopwords = append(target.Stopwords, strings.ToLower(value))
		}
	case "regexTarget":
		target.RegexTarget = values[0]
	case "condition":
		target.MatchAll = strings.EqualFold(values[0], "AND")
	case "description":
	}
	return nil
}

// arrayElement extracts one quoted element from a multiline array line
// (always `'''…''',` or `"…",` in the generated file).
func arrayElement(line string) (string, bool) {
	line = strings.TrimSuffix(strings.TrimSpace(line), ",")
	if line == "" {
		return "", false
	}
	return unquote(line), true
}

// splitInlineArray handles single-line arrays: ["a", "b-c", …].
func splitInlineArray(raw string) []string {
	inner := strings.TrimSuffix(strings.TrimPrefix(raw, "["), "]")
	var values []string
	for _, part := range strings.Split(inner, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, unquote(part))
		}
	}
	return values
}

func unquote(value string) string {
	if strings.HasPrefix(value, "'''") && strings.HasSuffix(value, "'''") && len(value) >= 6 {
		return value[3 : len(value)-3]
	}
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}
