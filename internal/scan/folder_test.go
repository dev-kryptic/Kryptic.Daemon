package scan

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanPathContextCanceledStopsBeforeReport(t *testing.T) {
	config := loadConfig(t)
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "app.py"), []byte("print(1)\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	_, err := config.ScanPathWithContext(ctx, directory, func(int, string) {
		cancel()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

func TestScanFolderWritesReportInSelectedFolderNotCwd(t *testing.T) {
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	target := t.TempDir()
	secret := `GITHUB_TOKEN = "ghp_16C7e42F292c6912E7710c838347Ae178B4a"`
	if err := os.WriteFile(filepath.Join(target, "settings.py"), []byte(secret), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := ScanFolder(context.Background(), target, nil, "test")
	if err != nil {
		t.Fatalf("ScanFolder: %v", err)
	}

	wantReport := filepath.Join(target, DefaultReportName)
	if result.ReportPath != wantReport {
		t.Fatalf("report path %s, want %s", result.ReportPath, wantReport)
	}
	if result.Files < 1 {
		t.Fatalf("files=%d", result.Files)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("findings=%d, want 1: %+v", len(result.Findings), result.Findings)
	}

	if _, err := os.Stat(filepath.Join(cwd, DefaultReportName)); !os.IsNotExist(err) {
		t.Fatal("report was written to the process CWD, not the selected folder")
	}

	data, err := os.ReadFile(wantReport)
	if err != nil {
		t.Fatal(err)
	}
	markdown := string(data)
	if strings.Contains(markdown, "ghp_16C7e42F") {
		t.Fatal("report leaked the raw secret")
	}
	if !strings.Contains(markdown, "This scan ran fully offline") {
		t.Fatal("report should state the scan ran fully offline")
	}
	if !strings.Contains(markdown, "settings.py") {
		t.Fatal("report is missing the finding file")
	}
}

func TestScanFolderCanceledDoesNotWriteReport(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "app.py"), []byte("print(1)\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	_, err := ScanFolder(ctx, directory, func(int, string) {
		cancel()
	}, "test")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
	if _, err := os.Stat(filepath.Join(directory, DefaultReportName)); !os.IsNotExist(err) {
		t.Fatal("cancelled scan left a report as if it completed")
	}
}

func TestLineProgressWritesPercentAndMessage(t *testing.T) {
	var buf strings.Builder
	progress := LineProgress(&buf)
	progress(12, "src/app.py")
	progress(100, "Done")
	got := buf.String()
	if !strings.Contains(got, "12\tsrc/app.py\n") {
		t.Fatalf("missing 12%% line: %q", got)
	}
	if !strings.Contains(got, "100\tDone\n") {
		t.Fatalf("missing 100%% line: %q", got)
	}
}
