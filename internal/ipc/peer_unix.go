//go:build linux || darwin

package ipc

import (
	"errors"
	"fmt"
	"net"
	"os"
)

// CheckPeer authenticates the connecting process by its OS credentials: the
// peer must run as the same user id as the daemon. The socket is also chmod
// 0600, but that is a filesystem side effect; the peer-credential check is the
// authoritative trust boundary. It defends against another local user reaching
// the socket (e.g. a shared /tmp on a multi-user host, or the brief window while
// the socket is being created) and makes "same user only" an explicit, enforced
// contract rather than an implicit assumption.
func CheckPeer(conn net.Conn) error {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return errors.New("connection is not a unix socket")
	}

	raw, err := unixConn.SyscallConn()
	if err != nil {
		return err
	}

	var (
		peerUID uint32
		readErr error
	)
	if controlErr := raw.Control(func(fd uintptr) {
		peerUID, readErr = readPeerUID(fd)
	}); controlErr != nil {
		return controlErr
	}
	if readErr != nil {
		return readErr
	}

	if selfUID := os.Getuid(); selfUID >= 0 && peerUID != uint32(selfUID) {
		return fmt.Errorf("rejected peer uid %d (daemon runs as uid %d)", peerUID, selfUID)
	}
	return nil
}
