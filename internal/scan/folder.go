package scan

import (
	"context"
	"path/filepath"
	"time"
)

// FolderResult is one offline folder scan plus the Markdown report it wrote.
type FolderResult struct {
	Root       string
	ReportPath string
	Files      int
	Findings   []Finding
}

// ScanFolder walks root with the embedded gitleaks engine and writes
// DefaultReportName at that folder's root (not the process CWD). The report
// is written only after a complete scan. A cancelled ctx returns without
// creating or replacing the report.
func ScanFolder(ctx context.Context, root string, progress Progress, version string) (FolderResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return FolderResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return FolderResult{}, err
	}

	config, err := Load()
	if err != nil {
		return FolderResult{}, err
	}

	result, err := config.ScanPathWithContext(ctx, abs, progress)
	if err != nil {
		return FolderResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return FolderResult{}, err
	}

	reportPath, err := ResolveExportPath(abs)
	if err != nil {
		return FolderResult{}, err
	}
	meta := ExportMeta{
		Target:    abs,
		Files:     result.Files,
		Rules:     len(config.Rules),
		Generated: time.Now(),
		Version:   version,
	}
	if err := WriteReport(reportPath, result.Findings, meta); err != nil {
		return FolderResult{}, err
	}
	return FolderResult{
		Root:       abs,
		ReportPath: reportPath,
		Files:      result.Files,
		Findings:   result.Findings,
	}, nil
}
