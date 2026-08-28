//go:build !linux

package main

// ensureLauncherIcon is Linux-only: macOS and Windows installers own their
// icons.
func ensureLauncherIcon() {}
