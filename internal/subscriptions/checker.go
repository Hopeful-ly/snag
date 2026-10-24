package subscriptions

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/Roham1299/snag/internal/playlist"
)

// failsToSay is how many consecutive failures it takes before the error is reported.
const (
	lookEvery   = time.Minute
	atOnce      = 2
	readTimeout = 5 * time.Minute
	failsToSay  = 3
)

// ErrOffline is returned by Read when the site is unreachable; the check is postponed by
// offlineWait without counting as a failure.
var ErrOffline = errors.New("offline")

const offlineWait = 15 * time.Minute

// Checker re-reads each subscription when it is due and downloads its new entries.
type Checker struct {
	Store *Store
	Read  func(ctx context.Context, s Subscription) (playlist.Playlist, error)
	// Add downloads the entries of p at indexes, in order; added excludes ones already queued.
	Add func(s Subscription, p playlist.Playlist, indexes []int) (added int, err error)
	// Told receives the titles of each check that added downloads.
	Told func(s Subscription, titles []string)
	Now  func() time.Time // nil for time.Now
	Log  *slog.Logger     // nil for slog.Default()

	mu      sync.Mutex
	reading map[string]bool
	wake    chan struct{}
}

func (c *Checker) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Checker) log() *slog.Logger {
	if c.Log != nil {
		return c.Log
	}
	return slog.Default()
}

func (c *Checker) wakeChan() chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.wake == nil {
		c.wake = make(chan struct{}, 1)
	}
	return c.wake
}

// Run checks for due subscriptions every lookEvery, after an initial wait, until ctx ends.
// CheckNow cuts the wait short.
func (c *Checker) Run(ctx context.Context, wait time.Duration) {
	wake := c.wakeChan()
	if wait > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		case <-wake:
		}
	}
	tick := time.NewTicker(lookEvery)
	defer tick.Stop()
	for {
		c.Tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-wake:
		}
	}
}

// Tick starts checking due subscriptions, at most atOnce at a time.
func (c *Checker) Tick(ctx context.Context) {
	now := c.now()
	c.mu.Lock()
	if c.reading == nil {
		c.reading = map[string]bool{}
	}
	var due []Subscription
	for _, s := range c.Store.List() {
		if s.Due(now) && !c.reading[s.ID] {
			due = append(due, s)
		}
	}
	slices.SortFunc(due, func(a, b Subscription) int { return a.Next.Compare(b.Next) })
	for _, s := range due {
		if len(c.reading) >= atOnce {
			break
		}
		c.reading[s.ID] = true
		go func() {
			c.Check(ctx, s)
			c.mu.Lock()
			delete(c.reading, s.ID)
			c.mu.Unlock()
			select { // let the next due one start without waiting for the next tick
			case c.wakeChan() <- struct{}{}:
			default:
			}
		}()
	}
	c.mu.Unlock()
}

// CheckNow makes a subscription due now and wakes the checker.
func (c *Checker) CheckNow(id string) error {
	now := c.now()
	if _, err := c.Store.Update(id, func(s *Subscription) { s.Next, s.Paused = now, false }); err != nil {
		return err
	}
	select {
	case c.wakeChan() <- struct{}{}:
	default:
	}
	return nil
}

// Busy reports whether a subscription is being checked.
func (c *Checker) Busy(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reading[id]
}

// Check downloads s's unseen entries that pass its filters, oldest first. Filtered entries are
// marked seen; unfinished premieres and live streams are not, so they download once finished.
func (c *Checker) Check(ctx context.Context, s Subscription) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	p, err := c.Read(ctx, s)
	now := c.now()
	if ctx.Err() != nil && errors.Is(err, context.Canceled) {
		return // shutting down: don't record a failure
	}
	if err != nil {
		c.failed(s.ID, err, now)
		return
	}

	seen := make(map[string]bool, len(s.Seen))
	for _, k := range s.Seen {
		seen[k] = true
	}
	var fresh, skipped []int
	for i, e := range p.Entries {
		k := EntryKey(e)
		switch {
		case seen[k] || e.Waiting:
		case !s.Filters.Lets(e):
			skipped = append(skipped, i)
		default:
			fresh = append(fresh, i)
		}
		seen[k] = true
	}
	if p.NewestFirst {
		slices.Reverse(fresh)
	}
	added := 0
	if len(fresh) > 0 {
		added, err = c.Add(s, p, fresh)
		if err != nil {
			c.failed(s.ID, err, now)
			return
		}
	}
	keys := make([]string, 0, len(fresh)+len(skipped))
	for _, i := range slices.Concat(fresh, skipped) {
		keys = append(keys, EntryKey(p.Entries[i]))
	}
	kept, err := c.Store.Update(s.ID, func(s *Subscription) {
		s.see(keys)
		s.Checked, s.Next, s.Error, s.Fails = now, s.after(now), "", 0
		s.Added += added
		s.Last = added
	})
	if err != nil {
		return // removed meanwhile
	}
	c.log().Info("looked at a subscription", "title", s.Title, "new", len(fresh), "added", added)
	if added > 0 && c.Told != nil {
		titles := make([]string, 0, len(fresh))
		for _, i := range fresh {
			titles = append(titles, p.Entries[i].Title)
		}
		c.Told(kept, titles)
	}
}

func (c *Checker) failed(id string, err error, now time.Time) {
	if errors.Is(err, ErrOffline) {
		_, _ = c.Store.Update(id, func(s *Subscription) { s.Next = now.Add(offlineWait) })
		return
	}
	s, _ := c.Store.Update(id, func(s *Subscription) {
		s.Fails++
		s.Checked, s.Next = now, s.retryAfter(now, s.Fails)
		if s.Fails >= failsToSay {
			s.Error = err.Error()
		}
	})
	c.log().Warn("couldn't look at a subscription", "title", s.Title, "fails", s.Fails, "err", err)
}
