package manageui

import "sync"

var (
	sessionMu   sync.Mutex
	sessionDone chan struct{}
)

func beginSession() {
	sessionMu.Lock()
	if sessionDone != nil {
		sessionMu.Unlock()
		return
	}
	sessionDone = make(chan struct{})
	sessionMu.Unlock()
}

func endSession() {
	sessionMu.Lock()
	if sessionDone != nil {
		close(sessionDone)
		sessionDone = nil
	}
	sessionMu.Unlock()
}

// Wait blocks until Open Kryptic closes. `kryptic panel` uses it.
func Wait() {
	sessionMu.Lock()
	ch := sessionDone
	sessionMu.Unlock()
	if ch != nil {
		<-ch
	}
}
