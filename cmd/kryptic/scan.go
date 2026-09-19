package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/dev-kryptic/daemon/internal/scan"
	"github.com/dev-kryptic/daemon/internal/server"
)

func runScan() error {
	config, err := scan.Load()
	if err != nil {
		return err
	}

	opts, err := scan.ParseArgs(os.Args[2:])
	if err != nil {
		return err
	}

	progress := scan.TerminalProgress()
	if opts.Progress {
		progress = scan.LineProgress(os.Stderr)
	}
	var findings []scan.Finding
	files := 0
	target := opts.Root
	if opts.Staged {
		diff, err := stagedDiff()
		if err != nil {
			return err
		}
		target = "(staged git index)"
		findings = config.ScanContentWithProgress("(staged diff)", diff, progress)
		files = 1
	} else {
		result, err := config.ScanPathWithProgress(opts.Root, progress)
		if err != nil {
			return err
		}
		findings = result.Findings
		files = result.Files
	}

	found := scan.Report(findings)

	if opts.Export {
		path, err := scan.ResolveExportPath(opts.ExportPath)
		if err != nil {
			return err
		}
		meta := scan.ExportMeta{
			Target:    target,
			Staged:    opts.Staged,
			Files:     files,
			Rules:     len(config.Rules),
			Generated: time.Now(),
			Version:   server.Version,
		}
		if err := scan.WriteReport(path, findings, meta); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", path)
	}

	if found {
		os.Exit(1)
	}
	return nil
}

func stagedDiff() (string, error) {
	output, err := exec.Command("git", "diff", "--cached", "--unified=0").Output()
	if err != nil {
		return "", fmt.Errorf("git diff --cached failed - is this a git repository?")
	}

	var added []string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added = append(added, strings.TrimPrefix(line, "+"))
		}
	}
	return strings.Join(added, "\n"), nil
}
