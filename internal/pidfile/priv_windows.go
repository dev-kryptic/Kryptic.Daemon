//go:build windows

package pidfile

func RefuseRoot(string) error { return nil }
