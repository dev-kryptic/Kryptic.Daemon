// Package dotenv renders KEY=value lines and filters keys that are not
// valid POSIX shell identifiers. CI and `kryptic secrets export` share it.
package dotenv

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// keyPattern is the POSIX shell identifier shape. A key outside it (spaces,
// newlines, `$(...)`) cannot be rendered safely on the left of a dotenv or
// shell `export` line: it would inject extra lines or commands into whatever
// sources the output. JSON export keeps every key; the encoder escapes them.
var keyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Pair struct {
	Key   string
	Value string
}

func ValidKey(key string) bool {
	return keyPattern.MatchString(key)
}

// Line renders one KEY=value line, quoting when the value contains
// whitespace or shell-significant characters.
func Line(key, value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\r\n#\"'`$\\") {
		return key + "=" + value
	}
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return key + `="` + escaped + `"`
}

func WarnSkipped(skipped []string) {
	if len(skipped) == 0 {
		return
	}
	quoted := make([]string, len(skipped))
	for i, key := range skipped {
		quoted[i] = strconv.Quote(key)
	}
	fmt.Fprintf(os.Stderr, "warning: skipped %d key(s) that are not valid shell identifiers: %s\n",
		len(quoted), strings.Join(quoted, ", "))
}

// Filter drops keys that are not valid shell identifiers and warns on stderr.
func Filter(pairs []Pair) []Pair {
	kept := make([]Pair, 0, len(pairs))
	var skipped []string
	for _, pair := range pairs {
		if ValidKey(pair.Key) {
			kept = append(kept, pair)
		} else {
			skipped = append(skipped, pair.Key)
		}
	}
	WarnSkipped(skipped)
	return kept
}

func PrintDotenv(pairs []Pair) {
	for _, pair := range Filter(pairs) {
		fmt.Println(Line(pair.Key, pair.Value))
	}
}

func PrintShell(pairs []Pair) {
	for _, pair := range Filter(pairs) {
		fmt.Printf("export %s='%s'\n", pair.Key, strings.ReplaceAll(pair.Value, "'", `'\''`))
	}
}

func PrintJSON(pairs []Pair) error {
	object := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		object[pair.Key] = pair.Value
	}
	encoded, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}
