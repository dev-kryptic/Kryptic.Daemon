//go:build windows

package ipc

import (
	"net"
	"os"
	"time"

	winio "github.com/Microsoft/go-winio"
)

// Endpoint resolves the named pipe (PROTOCOL.md §Transport). KRYPTIC_SOCKET_PATH
// overrides it, mirroring the unix socket override.
func Endpoint() string {
	if override := os.Getenv("KRYPTIC_SOCKET_PATH"); override != "" {
		return override
	}
	return `\\.\pipe\kryptic-daemon`
}

func Listen() (net.Listener, error) {
	// SYSTEM, Administrators and the creator/owner only - the named-pipe
	// equivalent of the 0600 unix socket.
	config := &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GA;;;OW)",
	}
	return winio.ListenPipe(Endpoint(), config)
}

func Dial(timeout time.Duration) (net.Conn, error) {
	return winio.DialPipe(Endpoint(), &timeout)
}
