package termopen

import "testing"

func TestShellJoin(t *testing.T) {
	got := shellJoin([]string{"/opt/kryptic", "connector", "run", "--name", "local"})
	want := "'/opt/kryptic' 'connector' 'run' '--name' 'local'"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestQuoteWindows(t *testing.T) {
	if got := quoteWindows(`C:\Kryptic\kryptic.exe`); got != `C:\Kryptic\kryptic.exe` {
		t.Fatalf("plain %q", got)
	}
	got := quoteWindows(`C:\Program Files\Kryptic\kryptic.exe`)
	want := `"C:\Program Files\Kryptic\kryptic.exe"`
	if got != want {
		t.Fatalf("spaced %q", got)
	}
}
