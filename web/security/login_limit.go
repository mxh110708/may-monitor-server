package security

import (
	"sync"
	"time"
)

type loginWindow struct {
	count int
	until time.Time
}

// LoginLimiter bounds both attempts and bookkeeping. No usernames, credentials
// or ever-growing address history are retained. Failed allocation fails closed.
type LoginLimiter struct {
	mu      sync.Mutex
	windows map[string]loginWindow
}

func (l *LoginLimiter) Allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.windows == nil {
		l.windows = make(map[string]loginWindow)
	}
	w, found := l.windows[ip]
	if !found || !now.Before(w.until) {
		for key, item := range l.windows {
			if !now.Before(item.until) {
				delete(l.windows, key)
			}
		}
		if len(l.windows) >= 4096 {
			return false
		}
		w = loginWindow{until: now.Add(time.Minute)}
	}
	if w.count >= 10 {
		return false
	}
	w.count++
	l.windows[ip] = w
	return true
}
