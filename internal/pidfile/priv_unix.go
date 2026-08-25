//go:build !windows

package pidfile

import (
	"fmt"
	"os"
)

// RefuseRoot returns an error when the process is running as root. The daemon
// is per-user: a root-owned socket (0600) is unreachable from the menu bar
// and from a normal `kryptic` CLI, which then looks stuck at "starting…".
func RefuseRoot(command string) error {
	if os.Geteuid() != 0 {
		return nil
	}
	return fmt.Errorf("refusing to %s as root. The daemon must run as your user so the menu bar and CLI can reach it. Run `kryptic %s` without sudo", command, command)
}
