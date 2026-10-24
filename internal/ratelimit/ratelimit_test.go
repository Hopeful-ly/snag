package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestReadersTogetherKeepToTheRate(t *testing.T) {
	const rate = 2 << 20 // 2 MiB/s
	l := New(rate)
	start := time.Now()
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 32 { // 32 × 32 KiB = 1 MiB each, 4 MiB together
				if err := l.Wait(context.Background(), 32<<10); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Wait()
	took := time.Since(start)
	// 4 MiB at 2 MiB/s: about 2 s, less the first burst.
	if took < 1700*time.Millisecond || took > 3*time.Second {
		t.Errorf("4 MiB at 2 MiB/s took %v", took)
	}
}

func TestNoLimitLetsEverythingThrough(t *testing.T) {
	l := New(0)
	start := time.Now()
	for range 1000 {
		if err := l.Wait(context.Background(), 1<<20); err != nil {
			t.Fatal(err)
		}
	}
	if took := time.Since(start); took > 50*time.Millisecond {
		t.Errorf("no limit waited %v", took)
	}
	var zero Limiter
	if err := zero.Wait(context.Background(), 1<<30); err != nil {
		t.Fatal(err)
	}
}

func TestTheRateCanChange(t *testing.T) {
	l := New(64 << 10)
	_ = l.Wait(context.Background(), 64<<10) // the burst
	l.SetRate(64 << 20)
	start := time.Now()
	for range 16 {
		_ = l.Wait(context.Background(), 64<<10) // 1 MiB at 64 MiB/s
	}
	if took := time.Since(start); took > 300*time.Millisecond {
		t.Errorf("after raising the limit, 1 MiB took %v", took)
	}
	if l.Rate() != 64<<20 {
		t.Errorf("Rate = %d", l.Rate())
	}
}

func TestWaitingEndsWithItsContext(t *testing.T) {
	l := New(1024)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_ = l.Wait(ctx, 64<<10) // the burst
	if err := l.Wait(ctx, 1<<20); err == nil {
		t.Error("a wait of a thousand seconds ended without its context")
	}
}
