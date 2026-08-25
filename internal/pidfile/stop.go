package pidfile

import (
	"errors"
	"fmt"
	"time"
)

// StopRunning terminates the daemon recorded in the pidfile and waits for it
// to exit. It does not touch the login session in the OS credential store.
func StopRunning() error {
	pid, err := Read()
	if err != nil {
		if errors.Is(err, ErrNotRunning) {
			return nil
		}
		return err
	}
	if !Alive(pid) {
		Clear()
		return nil
	}
	if err := Terminate(pid); err != nil {
		return fmt.Errorf("could not stop daemon (pid %d): %w", pid, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !Alive(pid) {
			Clear()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("daemon (pid %d) did not exit within 5s", pid)
}
