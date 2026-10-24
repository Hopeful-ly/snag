// Package queues runs download queues by the clock: each one with a schedule starts at its start
// time on its days, and pauses at its pause time if it has one.
package queues

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/Roham1299/snag/internal/config"
)

const checkEvery = 30 * time.Second

// A schedule without a pause time counts as running this long after its start: long enough to
// see it started after a late boot, short enough to start afresh the next day.
const openWindow = 12 * time.Hour

// Runner starts and stops queues: the download engine.
type Runner interface {
	StartQueue(id string)
	StopQueue(id string)
}

// Scheduler starts and stops queues as their schedules say. It acts only when a queue's schedule
// window opens or closes, so a queue started or stopped by hand stays so until then.
type Scheduler struct {
	run    Runner
	queues func() []config.Queue
	now    func() time.Time

	mu   sync.Mutex
	open map[string]bool // whether each queue's window was open when last looked at
}

// New returns a Scheduler for the queues the function gives, which may change.
func New(run Runner, queues func() []config.Queue) *Scheduler {
	return &Scheduler{run: run, queues: queues, now: time.Now, open: map[string]bool{}}
}

// RunAfter waits a while before Run: at login, so queues that should be running don't start
// downloading while the computer is still starting up.
func (s *Scheduler) RunAfter(ctx context.Context, wait time.Duration) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(wait):
	}
	s.Run(ctx)
}

// Run checks now and then every checkEvery until ctx ends.
func (s *Scheduler) Run(ctx context.Context) {
	tick := time.NewTicker(checkEvery)
	defer tick.Stop()
	for {
		s.Check()
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// Check starts the queues whose windows opened since it last looked, and stops those whose
// windows closed at their pause times. A queue whose window is open when Snag starts, or when its
// schedule is turned on, starts.
func (s *Scheduler) Check() {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[string]bool{}
	for _, q := range s.queues() {
		seen[q.ID] = true
		open := q.Schedule.On && Open(q.Schedule, now)
		was := s.open[q.ID]
		s.open[q.ID] = open
		switch {
		case open && !was:
			s.run.StartQueue(q.ID)
		case !open && was && q.Schedule.On && q.Schedule.Stop != "":
			s.run.StopQueue(q.ID)
		}
	}
	for id := range s.open {
		if !seen[id] {
			delete(s.open, id) // the queue was deleted
		}
	}
}

// Open reports whether t is within a run of the schedule: from a start time on one of its days
// until its pause time (the next day, for a pause time earlier than the start), or for openWindow
// when it has none.
func Open(sc config.Schedule, t time.Time) bool {
	start, ok := LastStart(sc, t)
	if !ok {
		return false
	}
	end := start.Add(openWindow)
	if stop, ok := config.ClockTime(sc.Stop); ok {
		end = time.Date(start.Year(), start.Month(), start.Day(), stop/60, stop%60, 0, 0, t.Location())
		if !end.After(start) {
			end = end.AddDate(0, 0, 1)
		}
	}
	return t.Before(end)
}

// LastStart is the schedule's latest start at or before t, within the past week.
func LastStart(sc config.Schedule, t time.Time) (time.Time, bool) {
	minutes, ok := config.ClockTime(sc.Start)
	if !ok {
		return time.Time{}, false
	}
	for back := range 8 {
		day := t.AddDate(0, 0, -back)
		start := time.Date(day.Year(), day.Month(), day.Day(), minutes/60, minutes%60, 0, 0, t.Location())
		if start.After(t) || !onDay(sc, start.Weekday()) {
			continue
		}
		return start, true
	}
	return time.Time{}, false
}

func onDay(sc config.Schedule, day time.Weekday) bool {
	return len(sc.Days) == 0 || slices.Contains(sc.Days, int(day))
}
