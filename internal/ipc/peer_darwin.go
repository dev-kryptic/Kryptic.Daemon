//go:build darwin

package ipc

import "golang.org/x/sys/unix"

// readPeerUID reads the connecting process's effective user id from the kernel
// via LOCAL_PEERCRED - the macOS/BSD equivalent of SO_PEERCRED. The credentials
// are captured at connect() time and cannot be forged by the peer.
func readPeerUID(fd uintptr) (uint32, error) {
	cred, err := unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
	if err != nil {
		return 0, err
	}
	return cred.Uid, nil
}
