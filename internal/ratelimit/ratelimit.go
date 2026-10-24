// Package ratelimit shares a speed limit between downloads: each waits before passing on what
// it read, so all of them together keep to the rate.
package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Limiter is a rate in bytes a second, shared by everyone who waits on it. The zero Limiter, or
// one with rate 0, lets everything through. It is safe for concurrent use.
type Limiter struct {
	mu     sync.Mutex
	rate   int64
	tokens float64 // what may pass now without waiting; negative when spoken for
	last   time.Time
	now    func() time.Time // for tests
}

// New returns a Limiter at rate bytes a second; 0 for none.
func New(rate int64) *Limiter {
	l := &Limiter{now: time.Now}
	l.SetRate(rate)
	l.tokens = l.burst() // the first reads go through at once
	return l
}

// SetRate changes the rate, for everyone waiting from now on; 0 for none.
func (l *Limiter) SetRate(rate int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refill()
	l.rate = max(rate, 0)
	l.tokens = min(l.tokens, l.burst())
}

// Rate is the rate in bytes a second; 0 for none.
func (l *Limiter) Rate() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rate
}

// Wait waits until n more bytes may pass, or ctx ends.
func (l *Limiter) Wait(ctx context.Context, n int) error {
	l.mu.Lock()
	if l.rate <= 0 {
		l.mu.Unlock()
		return nil
	}
	l.refill()
	l.tokens -= float64(n)
	wait := time.Duration(0)
	if l.tokens < 0 {
		wait = time.Duration(-l.tokens / float64(l.rate) * float64(time.Second))
	}
	l.mu.Unlock()
	if wait == 0 {
		return nil
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// refill adds what the time since the last look allows. The lock must be held.
func (l *Limiter) refill() {
	if l.now == nil {
		l.now = time.Now
	}
	now := l.now()
	if !l.last.IsZero() && l.rate > 0 {
		l.tokens = min(l.tokens+now.Sub(l.last).Seconds()*float64(l.rate), l.burst())
	}
	l.last = now
}

// burst is how much may pass at once after a pause: a quarter of a second's worth, so the speed
// is even, but at least a read's worth.
func (l *Limiter) burst() float64 {
	return max(float64(l.rate)/4, 64<<10)
}
