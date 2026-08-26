//go:build !linux && !windows

package main

func currentTrayIcon() []byte { return currentTrayPNG() }

func panelIsLight() bool { return false }

func watchTrayTheme() {}
