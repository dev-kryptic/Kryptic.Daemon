package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    Options
		wantErr bool
	}{
		{name: "defaults", want: Options{Root: "."}},
		{name: "path", args: []string{"src"}, want: Options{Root: "src"}},
		{name: "staged", args: []string{"--staged"}, want: Options{Root: ".", Staged: true}},
		{name: "export cwd", args: []string{"--export"}, want: Options{Root: ".", Export: true}},
		{
			name: "export path",
			args: []string{"--export", "out/report.md"},
			want: Options{Root: ".", Export: true, ExportPath: "out/report.md"},
		},
		{
			name: "export equals",
			args: []string{"--export=report.md"},
			want: Options{Root: ".", Export: true, ExportPath: "report.md"},
		},
		{
			name: "path then export",
			args: []string{"app", "--export"},
			want: Options{Root: "app", Export: true},
		},
		{
			name: "export then scan path",
			args: []string{"--export", "reports", "app"},
			want: Options{Root: "app", Export: true, ExportPath: "reports"},
		},
		{
			name: "staged and export",
			args: []string{"--staged", "--export"},
			want: Options{Root: ".", Staged: true, Export: true},
		},
		{name: "progress", args: []string{"--progress"}, want: Options{Root: ".", Progress: true}},
		{name: "unknown flag", args: []string{"--nope"}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseArgs(test.args)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseArgs: %v", err)
			}
			if got != test.want {
				t.Fatalf("got %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestResolveExportPathEmptyUsesCwd(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	path, err := ResolveExportPath("")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(cwd, DefaultReportName)
	if path != want {
		t.Fatalf("got %s, want %s", path, want)
	}
}

func TestResolveExportPathDirectory(t *testing.T) {
	dir := t.TempDir()
	path, err := ResolveExportPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(dir, DefaultReportName) {
		t.Fatalf("got %s", path)
	}
}

func TestResolveExportPathFile(t *testing.T) {
	dir := t.TempDir()
	given := filepath.Join(dir, "custom.md")
	path, err := ResolveExportPath(given)
	if err != nil {
		t.Fatal(err)
	}
	if path != given {
		t.Fatalf("got %s, want %s", path, given)
	}
}

func TestResolveExportPathAddsMarkdownExtension(t *testing.T) {
	dir := t.TempDir()
	given := filepath.Join(dir, "custom")
	path, err := ResolveExportPath(given)
	if err != nil {
		t.Fatal(err)
	}
	if path != given+".md" {
		t.Fatalf("got %s", path)
	}
}

func TestRenderMarkdownIncludesLogoAndRedacts(t *testing.T) {
	secret := "ghp_16C7e42F292c6912E7710c838347Ae178B4a"
	markdown := RenderMarkdown([]Finding{{
		File:        "settings.py",
		Line:        14,
		RuleID:      "github-pat",
		Description: "Uncovered a GitHub Personal Access Token.",
		Secret:      secret,
	}}, ExportMeta{
		Target:    "/tmp/project",
		Files:     12,
		Rules:     222,
		Generated: time.Date(2026, 8, 31, 16, 20, 0, 0, time.UTC),
		Version:   "1.2.3",
	})

	if !strings.Contains(markdown, LogoURL) {
		t.Fatal("report is missing the logo URL")
	}
	if !strings.Contains(markdown, "settings.py") || !strings.Contains(markdown, "github-pat") {
		t.Fatal("report is missing finding details")
	}
	if strings.Contains(markdown, secret) {
		t.Fatal("report leaked the raw secret")
	}
	if !strings.Contains(markdown, "Action required") {
		t.Fatal("report should flag findings")
	}
	if !strings.Contains(markdown, "kryptic 1.2.3") {
		t.Fatal("report is missing the CLI version")
	}
}

func TestRenderMarkdownAllClear(t *testing.T) {
	markdown := RenderMarkdown(nil, ExportMeta{Files: 4, Rules: 222})
	if !strings.Contains(markdown, "All clear") {
		t.Fatal("empty scan should report all clear")
	}
	if !strings.Contains(markdown, LogoURL) {
		t.Fatal("clean report is missing the logo")
	}
}

func TestWriteReport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out", DefaultReportName)
	if err := WriteReport(path, nil, ExportMeta{Files: 1, Rules: 222, Version: "test"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), LogoURL) {
		t.Fatal("written report missing logo")
	}
}

func TestScanPathProgressReaches100(t *testing.T) {
	config := loadConfig(t)
	directory := t.TempDir()
	for _, name := range []string{"a.py", "b.py", "c.py", "d.py"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("print(1)\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var percents []int
	result, err := config.ScanPathWithProgress(directory, func(percent int, _ string) {
		percents = append(percents, percent)
	})
	if err != nil {
		t.Fatalf("ScanPathWithProgress: %v", err)
	}
	if result.Files != 4 {
		t.Fatalf("files=%d, want 4", result.Files)
	}
	if len(percents) == 0 || percents[0] != 0 {
		t.Fatalf("progress should start at 0, got %v", percents)
	}
	if percents[len(percents)-1] != 100 {
		t.Fatalf("progress should end at 100, got %v", percents)
	}
	for i := 1; i < len(percents); i++ {
		if percents[i] < percents[i-1] {
			t.Fatalf("progress went backwards: %v", percents)
		}
	}
}

func TestTruncateStatus(t *testing.T) {
	long := strings.Repeat("a", 80)
	got := truncateStatus(long, 10)
	if !strings.HasPrefix(got, "…") || len([]rune(got)) != 10 {
		t.Fatalf("got %q (len %d)", got, len([]rune(got)))
	}
	if truncateStatus("short", 10) != "short" {
		t.Fatal("short status should be unchanged")
	}
}
