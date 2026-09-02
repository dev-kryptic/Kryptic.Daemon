// Package dialog is the small native prompt layer used by the Windows and
// Linux tray (info, yes/no, text entry, progress, folder picker). macOS uses
// SwiftUI/AppKit instead.
package dialog

// Progress is a determinate 0-100 window. Close it when the work finishes.
// Canceled is closed when the user hits Cancel or closes the window. Close
// after a successful run does not treat the work as cancelled.
type Progress interface {
	Set(percent int, message string)
	Close()
	Canceled() <-chan struct{}
}

type nopProgress struct{}

func (nopProgress) Set(int, string)           {}
func (nopProgress) Close()                    {}
func (nopProgress) Canceled() <-chan struct{} { return nil }
