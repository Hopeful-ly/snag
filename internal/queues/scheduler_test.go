package queues

import (
	"slices"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/config"
)

// 2026-10-12 is a Monday.
func at(day int, hour, minute int) time.Time {
	return time.Date(2026, 10, day, hour, minute, 0, 0, time.Local)
}

func TestOpen(t *testing.T) {
	night := config.Schedule{On: true, Start: "02:00", Stop: "07:00"}
	overnight := config.Schedule{On: true, Start: "23:00", Stop: "06:00"}
	weekends := config.Schedule{On: true, Start: "10:00", Days: []int{0, 6}} // no pause time
	for _, tc := range []struct {
		name string
		sc   config.Schedule
		t    time.Time
		want bool
	}{
		{"before the start", night, at(12, 1, 59), false},
		{"at the start", night, at(12, 2, 0), true},
		{"during", night, at(12, 6, 59), true},
		{"at the pause", night, at(12, 7, 0), false},
		{"in the afternoon", night, at(12, 15, 0), false},
		{"overnight, before midnight", overnight, at(12, 23, 30), true},
		{"overnight, after midnight", overnight, at(13, 5, 0), true},
		{"overnight, after the pause", overnight, at(13, 6, 30), false},
		{"on a Saturday", weekends, at(17, 11, 0), true},
		{"on a Saturday night, without a pause time", weekends, at(17, 21, 59), true},
		{"twelve hours on", weekends, at(17, 22, 0), false},
		{"on a Monday", weekends, at(12, 11, 0), false},
		{"without a start", config.Schedule{On: true}, at(12, 11, 0), false},
	} {
		if got := Open(tc.sc, tc.t); got != tc.want {
			t.Errorf("%s: Open = %v, want %v", tc.name, got, tc.want)
		}
	}
}

type fakeRunner struct{ calls []string }

func (f *fakeRunner) StartQueue(id string) { f.calls = append(f.calls, "start "+id) }
func (f *fakeRunner) StopQueue(id string)  { f.calls = append(f.calls, "stop "+id) }

// A scheduler watching a clock the test moves.
func newScheduler(queues *[]config.Queue, now *time.Time) (*Scheduler, *fakeRunner) {
	run := &fakeRunner{}
	s := New(run, func() []config.Queue { return *queues })
	s.now = func() time.Time { return *now }
	return s, run
}

func TestQueuesStartAndPauseByTheClock(t *testing.T) {
	queues := []config.Queue{
		{ID: config.MainQueue},
		{ID: "night", Schedule: config.Schedule{On: true, Start: "02:00", Stop: "07:00"}},
		{ID: "games", Schedule: config.Schedule{On: true, Start: "03:00"}}, // runs till it's done
	}
	now := at(12, 1, 0)
	s, run := newScheduler(&queues, &now)
	step := func(t2 time.Time, want ...string) {
		t.Helper()
		run.calls = nil
		now = t2
		s.Check()
		if !slices.Equal(run.calls, want) {
			t.Errorf("at %s: %v, want %v", now.Format("Mon 15:04"), run.calls, want)
		}
	}
	step(at(12, 1, 0))
	step(at(12, 2, 0), "start night")
	step(at(12, 2, 30)) // still open: a queue stopped by hand meanwhile stays stopped
	step(at(12, 3, 0), "start games")
	step(at(12, 7, 0), "stop night")
	step(at(12, 15, 0)) // games has no pause time: nothing stops it
	step(at(13, 2, 0), "start night")
	step(at(13, 3, 0), "start games")
}

// Snag started in the middle of a window starts the queue, as after a late boot.
func TestAQueueInItsWindowStartsWithSnag(t *testing.T) {
	queues := []config.Queue{{ID: "night", Schedule: config.Schedule{On: true, Start: "02:00", Stop: "07:00"}}}
	now := at(12, 4, 0)
	s, run := newScheduler(&queues, &now)
	s.Check()
	if !slices.Equal(run.calls, []string{"start night"}) {
		t.Errorf("calls = %v", run.calls)
	}
}

// Turning a schedule off stops nothing (the queue was started; it may be running by hand), and
// turning it on within its window starts it.
func TestChangingASchedule(t *testing.T) {
	queues := []config.Queue{{ID: "night", Schedule: config.Schedule{On: false, Start: "02:00", Stop: "07:00"}}}
	now := at(12, 4, 0)
	s, run := newScheduler(&queues, &now)
	s.Check()
	queues[0].Schedule.On = true
	s.Check()
	queues[0].Schedule.On = false
	s.Check()
	if !slices.Equal(run.calls, []string{"start night"}) {
		t.Errorf("calls = %v", run.calls)
	}
}
