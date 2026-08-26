// Package dialog is the small native prompt layer used by the Windows and
// Linux tray (info, yes/no, text entry, progress). macOS uses SwiftUI/AppKit
// instead.
package dialog

// Progress is a determinate 0-100 window. Close it when the work finishes.
type Progress interface {
	Set(percent int, message string)
	Close()
}

type nopProgress struct{}

func (nopProgress) Set(int, string) {}
func (nopProgress) Close()          {}
