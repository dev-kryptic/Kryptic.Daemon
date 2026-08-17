package scan

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Finding is one detected secret.
type Finding struct {
	File        string
	Line        int
	RuleID      string
	Description string
	Secret      string
}

// Redacted shows enough of the secret to locate it without reprinting it.
func (f Finding) Redacted() string {
	if utf8.RuneCountInString(f.Secret) <= 8 {
		return "********"
	}
	runes := []rune(f.Secret)
	return string(runes[:4]) + strings.Repeat("*", len(runes)-4)
}

const maxFileSize = 1 << 20 // 1 MiB - larger files are generated artifacts, not source

// ScanPath walks a file or directory tree and returns all findings.
func (c *Config) ScanPath(root string) ([]Finding, error) {
	var findings []Finding

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			relative = path
		}
		if relative == "." {
			relative = filepath.Base(path) // root itself is a file, not a directory
		}
		relative = filepath.ToSlash(relative)

		if c.pathAllowlisted(relative) {
			return nil
		}
		if info, statErr := entry.Info(); statErr == nil && info.Size() > maxFileSize {
			return nil
		}

		fileFindings, scanErr := c.scanFile(path, relative)
		if scanErr != nil {
			return nil // unreadable file (permissions, vanished) - skip, don't abort the walk
		}
		findings = append(findings, fileFindings...)
		return nil
	})

	return findings, err
}

// ScanContent scans an in-memory blob (used for `kryptic scan --staged` diffs
// and piped input). name appears in findings. Matching runs against the whole
// content, exactly like gitleaks - several rules (private keys among them)
// span multiple lines.
func (c *Config) ScanContent(name, content string) []Finding {
	var findings []Finding
	lower := strings.ToLower(content)

	for ruleIndex := range c.Rules {
		rule := &c.Rules[ruleIndex]
		if rule.Regex == nil {
			continue
		}
		if rule.Path != nil && !rule.Path.MatchString(name) {
			continue
		}
		if !keywordHit(rule.Keywords, lower) {
			continue
		}

		for _, match := range rule.Regex.FindAllStringSubmatchIndex(content, -1) {
			wholeMatch := content[match[0]:match[1]]
			secret := wholeMatch
			if len(match) > 3 && match[2] >= 0 {
				secret = content[match[2]:match[3]]
			}
			if secret == "" {
				continue
			}

			if rule.Entropy > 0 && shannonEntropy(secret) < rule.Entropy {
				continue
			}

			line := lineOf(content, match[0])
			if c.allowlisted(rule, name, lineText(content, match[0]), wholeMatch, secret) {
				continue
			}

			findings = append(findings, Finding{
				File:        name,
				Line:        line,
				RuleID:      rule.ID,
				Description: rule.Description,
				Secret:      secret,
			})
		}
	}

	return dedupe(findings)
}

func (c *Config) scanFile(path, relative string) ([]Finding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if bytesLookBinary(data) {
		return nil, nil
	}
	return c.ScanContent(relative, string(data)), nil
}

func bytesLookBinary(data []byte) bool {
	probe := data
	if len(probe) > 8000 {
		probe = probe[:8000]
	}
	for _, b := range probe {
		if b == 0 {
			return true
		}
	}
	return false
}

// lineOf converts a byte offset into a 1-based line number.
func lineOf(content string, offset int) int {
	return 1 + strings.Count(content[:offset], "\n")
}

// lineText returns the full line containing the offset (allowlists with
// regexTarget = "line" match against it).
func lineText(content string, offset int) string {
	start := strings.LastIndexByte(content[:offset], '\n') + 1
	end := strings.IndexByte(content[offset:], '\n')
	if end < 0 {
		return content[start:]
	}
	return content[start : offset+end]
}

// dedupe collapses overlapping detections of the same secret at the same spot,
// preferring the specific rule over the generic catch-all.
func dedupe(findings []Finding) []Finding {
	type location struct {
		file   string
		line   int
		secret string
	}
	byLocation := map[location]int{}
	var result []Finding

	for _, finding := range findings {
		key := location{finding.File, finding.Line, finding.Secret}
		if existingIndex, seen := byLocation[key]; seen {
			if result[existingIndex].RuleID == "generic-api-key" {
				result[existingIndex] = finding
			}
			continue
		}
		byLocation[key] = len(result)
		result = append(result, finding)
	}
	return result
}

func keywordHit(keywords []string, lowerLine string) bool {
	if len(keywords) == 0 {
		return true
	}
	for _, keyword := range keywords {
		if strings.Contains(lowerLine, keyword) {
			return true
		}
	}
	return false
}

func (c *Config) pathAllowlisted(path string) bool {
	for _, pattern := range c.Global.Paths {
		if pattern.MatchString(path) {
			return true
		}
	}
	return false
}

func (c *Config) allowlisted(rule *Rule, file, line, wholeMatch, secret string) bool {
	// Global regex allowlist applies to the extracted secret.
	for _, pattern := range c.Global.Regexes {
		if pattern.MatchString(secret) {
			return true
		}
	}
	for _, stopword := range c.Global.Stopwords {
		if strings.Contains(strings.ToLower(secret), stopword) {
			return true
		}
	}

	for _, allowlist := range rule.Allowlists {
		if allowlistMatches(&allowlist, file, line, wholeMatch, secret) {
			return true
		}
	}
	return false
}

// allowlistMatches implements gitleaks allowlist semantics: with the default
// OR condition any populated criterion suppresses the finding; with AND all
// populated criteria must hit.
func allowlistMatches(allowlist *Allowlist, file, line, wholeMatch, secret string) bool {
	target := secret
	switch allowlist.RegexTarget {
	case "match":
		target = wholeMatch
	case "line":
		target = line
	}

	pathHit := anyMatch(allowlist.Paths, file)
	regexHit := anyMatch(allowlist.Regexes, target)
	stopwordHit := false
	lowerSecret := strings.ToLower(secret)
	for _, stopword := range allowlist.Stopwords {
		if strings.Contains(lowerSecret, stopword) {
			stopwordHit = true
			break
		}
	}

	if allowlist.MatchAll {
		result := true
		if len(allowlist.Paths) > 0 {
			result = result && pathHit
		}
		if len(allowlist.Regexes) > 0 {
			result = result && regexHit
		}
		if len(allowlist.Stopwords) > 0 {
			result = result && stopwordHit
		}
		return result && (len(allowlist.Paths)+len(allowlist.Regexes)+len(allowlist.Stopwords) > 0)
	}
	return pathHit || regexHit || stopwordHit
}

func anyMatch(patterns []*regexp.Regexp, value string) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}

// shannonEntropy in bits per character - the same measure gitleaks uses.
func shannonEntropy(value string) float64 {
	if value == "" {
		return 0
	}
	counts := map[rune]int{}
	for _, r := range value {
		counts[r]++
	}
	entropy := 0.0
	length := float64(len([]rune(value)))
	for _, count := range counts {
		p := float64(count) / length
		entropy -= p * math.Log2(p)
	}
	return entropy
}

// Report prints findings in the CLI format and returns an exit hint.
func Report(findings []Finding) bool {
	if len(findings) == 0 {
		fmt.Println("no secrets found.")
		return false
	}
	for _, finding := range findings {
		fmt.Printf("%s:%d  %s  %s\n    %s\n", finding.File, finding.Line, finding.RuleID, finding.Redacted(), finding.Description)
	}
	fmt.Printf("\n%d potential secret(s) found.\n", len(findings))
	return true
}
