//go:build windows

package ipc

import "net"

// CheckPeer is a no-op on Windows: the named pipe is created with a security
// descriptor (see Listen) that already restricts access to SYSTEM,
// Administrators and the pipe's creator/owner, so only the same user can
// connect. There is no cross-user window to close.
func CheckPeer(conn net.Conn) error {
	return nil
}
