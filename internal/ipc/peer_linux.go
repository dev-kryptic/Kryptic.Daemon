//go:build linux

package ipc

import "golang.org/x/sys/unix"

// readPeerUID reads the connecting process's user id from the kernel via
// SO_PEERCRED - the credentials are recorded at connect() time and cannot be
// forged by the peer.
func readPeerUID(fd uintptr) (uint32, error) {
	cred, err := unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	if err != nil {
		return 0, err
	}
	return cred.Uid, nil
}
