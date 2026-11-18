package bunq

import (
	"context"
	"sync"
	"time"
)

// limiter spaces requests at least interval apart.
type limiter struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func newLimiter(interval time.Duration) *limiter {
	return &limiter{interval: interval}
}

func (l *limiter) wait(ctx context.Context) error {
	l.mu.Lock()
	now := time.Now()
	at := l.next
	if at.Before(now) {
		at = now
	}
	l.next = at.Add(l.interval)
	l.mu.Unlock()

	if d := time.Until(at); d > 0 {
		return sleep(ctx, d)
	}
	return nil
}
