//go:build linux || darwin

package ipc

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

// A connection from the same user (the test process itself) must pass the
// peer-credential check - this exercises the real getsockopt path on the host.
func TestCheckPeerAllowsSameUser(t *testing.T) {
	// A short relative name resolved against the current directory: the unix
	// socket sun_path limit (104 bytes on macOS) is too small for the deeply
	// nested go test temp dir.
	socketPath := fmt.Sprintf("peer-test-%d.sock", os.Getpid())
	_ = os.Remove(socketPath)
	defer os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	accepted := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			accepted <- acceptErr
			return
		}
		defer conn.Close()
		accepted <- CheckPeer(conn)
	}()

	client, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	select {
	case err := <-accepted:
		if err != nil {
			t.Fatalf("CheckPeer rejected a same-user connection: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for accept")
	}
}

// A non-unix connection must be refused rather than mistaken for a trusted peer.
func TestCheckPeerRejectsNonUnix(t *testing.T) {
	clientEnd, serverEnd := net.Pipe()
	defer clientEnd.Close()
	defer serverEnd.Close()

	if err := CheckPeer(serverEnd); err == nil {
		t.Fatal("CheckPeer accepted a non-unix connection, want error")
	}
}
