//go:build !linux

package update

import "fmt"

func privilegedInstall(pairs [][2]string) error {
	if len(pairs) == 0 {
		return nil
	}
	return fmt.Errorf("cannot write %s (permission denied)", pairs[0][1])
}
