package main

import "testing"

func TestParseGSettingsColorScheme(t *testing.T) {
	tests := []struct {
		in        string
		light, ok bool
	}{
		{"'prefer-dark'", false, true},
		{"prefer-dark", false, true},
		{"'prefer-light'", true, true},
		{"'default'", false, true},
		{"", false, false},
		{"'bogus'", false, false},
	}
	for _, tt := range tests {
		light, ok := parseGSettingsColorScheme(tt.in)
		if light != tt.light || ok != tt.ok {
			t.Fatalf("%q: got light=%v ok=%v", tt.in, light, ok)
		}
	}
}

func TestGtkThemeIsLight(t *testing.T) {
	if gtkThemeIsLight("") {
		t.Fatal("empty theme should be treated as dark")
	}
	if gtkThemeIsLight("Yaru-dark") {
		t.Fatal("Yaru-dark")
	}
	if gtkThemeIsLight("Adwaita:dark") {
		t.Fatal("Adwaita:dark")
	}
	if !gtkThemeIsLight("Yaru") {
		t.Fatal("Yaru")
	}
}
