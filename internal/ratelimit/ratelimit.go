// Package ratelimit provides a fixed-window, per-key request limiter.
package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Limiter allows up to a fixed number of hits per key in each window. The
// window for a key starts at its first hit. It is safe for concurrent use.
type Limiter struct {
	limit  int
	window time.Duration
	now    func() time.Time

	mu   sync.Mutex
	hits map[string]bucket
}

type bucket struct {
	count int
	reset time.Time
}

// New returns a limiter allowing limit hits per key in each window. now is
// the clock; pass nil to use time.Now.
func New(limit int, window time.Duration, now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{limit: limit, window: window, now: now, hits: map[string]bucket{}}
}

// Allow records a hit for key and reports whether it is within the limit.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b, ok := l.hits[key]
	if !ok || now.After(b.reset) {
		l.hits[key] = bucket{count: 1, reset: now.Add(l.window)}
		return true
	}
	if b.count >= l.limit {
		return false
	}
	b.count++
	l.hits[key] = b
	return true
}

// Prune forgets keys whose window has ended, so memory stays bounded.
func (l *Limiter) Prune() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for key, b := range l.hits {
		if now.After(b.reset) {
			delete(l.hits, key)
		}
	}
}

// Run prunes expired keys once per window until ctx is cancelled.
func (l *Limiter) Run(ctx context.Context) {
	t := time.NewTicker(l.window)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			l.Prune()
		}
	}
}

// size returns the number of tracked keys (for tests).
func (l *Limiter) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.hits)
}
