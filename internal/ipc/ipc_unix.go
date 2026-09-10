//go:build !windows

package ipc

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/sys/unix"

	"github.com/dev-kryptic/daemon/internal/config"
)

// Endpoint resolves the well-known unix socket path (PROTOCOL.md §Transport)
// for display; endpoint is the fallible variant Listen and Dial use.
func Endpoint() string {
	path, _ := endpoint()
	return path
}

// endpoint places the socket in a per-user directory. Never /tmp: a
// predictable path in a world-writable directory lets any local user squat
// on or swap the socket. When no per-user directory can be resolved, the
// caller fails instead of degrading to a shared path.
func endpoint() (string, error) {
	if override := os.Getenv("KRYPTIC_SOCKET_PATH"); override != "" {
		return override, nil
	}
	dir, err := socketDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "kryptic-daemon.sock"), nil
}

// socketDir is $XDG_RUNTIME_DIR on Linux (already per-user and 0700).
// Everywhere else it reuses the daemon's per-user config directory
// (~/Library/Application Support/kryptic on macOS, ~/.config/kryptic on
// Linux), which config creates 0700.
func socketDir() (string, error) {
	if runtime.GOOS == "linux" {
		if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
			return runtimeDir, nil
		}
	}
	dir, err := config.Dir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve a per-user directory for the daemon socket: %w", err)
	}
	return dir, nil
}

func Listen() (net.Listener, error) {
	path, err := endpoint()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
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
	path, err := endpoint()
	if err != nil {
		return nil, err
	}
	// Mirror image of the server's peer-credential check: refuse to send
	// requests into a socket another local user planted at the expected path.
	var info unix.Stat_t
	if err := unix.Stat(path, &info); err != nil {
		return nil, err
	}
	if info.Uid != uint32(os.Getuid()) {
		return nil, fmt.Errorf(
			"socket %s is owned by uid %d, not the current user (uid %d) - refusing to connect",
			path, info.Uid, os.Getuid())
	}
	return net.DialTimeout("unix", path, timeout)
}
