package applog

import "os"

// Reveal opens the diagnostics folder in the platform file manager so a user
// can attach kryptic.krypticlog to a support request.
func Reveal() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// Touch the current file so Reveal has something to select.
	path, err := Path()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		Event("kryptic", "logs.created")
	}
	return revealPath(path)
}
