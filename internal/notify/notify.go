package notify

import (
	"strings"
	"sync"
	"time"
)

const cooldown = 15 * time.Minute

var (
	mu       sync.Mutex
	lastSent = map[string]time.Time{}
)

// Alert shows a non-blocking OS notification. The same key is suppressed for
// 15 minutes so a polling SDK cannot flood the desktop.
func Alert(key, title, body string) {
	key = strings.TrimSpace(key)
	if key == "" {
		key = title + "\n" + body
	}

	mu.Lock()
	if time.Since(lastSent[key]) < cooldown {
		mu.Unlock()
		return
	}
	lastSent[key] = time.Now()
	mu.Unlock()

	go show(title, body)
}
