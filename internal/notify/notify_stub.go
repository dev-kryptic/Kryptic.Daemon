//go:build !darwin && !linux && !windows

package notify

func show(title, body string) {}
