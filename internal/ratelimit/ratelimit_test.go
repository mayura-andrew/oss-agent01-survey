package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func TestAllowWindow(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	l := New(3, time.Minute, clk.Now)

	for i := 1; i <= 3; i++ {
		if !l.Allow("a") {
			t.Fatalf("hit %d blocked", i)
		}
	}
	if l.Allow("a") {
		t.Fatal("hit 4 allowed")
	}
	if !l.Allow("b") {
		t.Fatal("other key blocked")
	}

	clk.Advance(time.Minute) // exactly at reset: still the same window
	if l.Allow("a") {
		t.Fatal("allowed at the window boundary")
	}
	clk.Advance(time.Nanosecond)
	if !l.Allow("a") {
		t.Fatal("not reset after window")
	}
}

func TestPrune(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	l := New(1, time.Minute, clk.Now)
	l.Allow("a")
	clk.Advance(30 * time.Second)
	l.Allow("b")
	clk.Advance(31 * time.Second)
	l.Prune()
	if n := l.size(); n != 1 {
		t.Fatalf("tracked %d keys after prune, want 1", n)
	}
}

func TestDefaultClock(t *testing.T) {
	l := New(1, time.Hour, nil)
	if !l.Allow("a") || l.Allow("a") {
		t.Fatal("default clock limiter misbehaves")
	}
}

func TestRunStops(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	l := New(1, time.Millisecond, clk.Now)
	l.Allow("a")
	clk.Advance(time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { l.Run(ctx); close(done) }()

	deadline := time.Now().Add(2 * time.Second)
	for l.size() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("Run never pruned")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after cancel")
	}
}
