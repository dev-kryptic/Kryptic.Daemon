//go:build !windows

package ipc

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/sys/unix"
)

// Endpoint resolves the well-known unix socket path (PROTOCOL.md §Transport).
func Endpoint() string {
	if override := os.Getenv("KRYPTIC_SOCKET_PATH"); override != "" {
		return override
	}
	if runtime.GOOS == "linux" {
		if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
			return filepath.Join(runtimeDir, "kryptic-daemon.sock")
		}
	}
	return "/tmp/kryptic-daemon.sock"
}

func Listen() (net.Listener, error) {
	path := Endpoint()
	_ = os.Remove(path) // a previous daemon may have crashed without cleanup

	// net.Listen creates the socket file honoring the process umask. Forcing
	// 0177 here means the socket is never even briefly reachable by group/other,
	// closing the race between Listen and the Chmod below. The peer-credential
	// check in CheckPeer is the authoritative gate; these are defense in depth.
	oldMask := unix.Umask(0o177)
	listener, err := net.Listen("unix", path)
	unix.Umask(oldMask)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(path, 0o600) // only the current OS user talks to the daemon
	return listener, nil
}

func Dial(timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", Endpoint(), timeout)
}
