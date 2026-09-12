//go:build darwin

package manageui

// Show is a no-op on macOS. The Swift menu-bar app hosts Open Kryptic
// (ManageWindow.swift).
func Show(Handlers) {}
