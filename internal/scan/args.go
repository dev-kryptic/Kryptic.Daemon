package scan

import (
	"fmt"
	"strings"
)

// Options is the parsed `kryptic scan` command line.
type Options struct {
	Root       string
	Staged     bool
	Export     bool
	ExportPath string
}

// ParseArgs reads `kryptic scan` flags. `--export` with no path writes the
// report into the current working directory.
func ParseArgs(args []string) (Options, error) {
	opts := Options{Root: "."}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--staged":
			opts.Staged = true
		case arg == "--export":
			opts.Export = true
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				opts.ExportPath = args[i]
			}
		case strings.HasPrefix(arg, "--export="):
			opts.Export = true
			opts.ExportPath = strings.TrimPrefix(arg, "--export=")
		case strings.HasPrefix(arg, "-"):
			return Options{}, fmt.Errorf("unknown flag %s (usage: kryptic scan [PATH] [--staged] [--export [FILE|DIR]])", arg)
		default:
			opts.Root = arg
		}
	}
	return opts, nil
}
