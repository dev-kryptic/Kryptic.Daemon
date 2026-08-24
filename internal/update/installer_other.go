//go:build !windows

package update

func openWindowsInstaller(path string) error {
	return nil
}
