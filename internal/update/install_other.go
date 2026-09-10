//go:build !linux

package update

import "fmt"

func privilegedInstall(files []stagedFile) error {
	if len(files) == 0 {
		return nil
	}
	return fmt.Errorf("cannot write %s (permission denied)", files[0].dest)
}
