package service

import (
	"crypto/rand"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/config"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/httpdl/httpdltest"
)

// actions records when-done actions instead of shutting the computer down.
type actions struct {
	mu   sync.Mutex
	done []string
}

func recordActions(t *testing.T) *actions {
	a := &actions{}
	old, oldTime := whenDone, countdownTime
	whenDone = func(action string, quit func()) error {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.done = append(a.done, action)
		return nil
	}
	countdownTime = 300 * time.Millisecond
	t.Cleanup(func() { whenDone, countdownTime = old, oldTime })
	return a
}

func (a *actions) list() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.done...)
}

func TestAFinishedQueueDoesWhatItWasSetTo(t *testing.T) {
	acts := recordActions(t)
	s := startService(t)
	var mu sync.Mutex
	var heard []*Countdown
	s.OnCountdown(func(c *Countdown) { mu.Lock(); heard = append(heard, c); mu.Unlock() })

	night, err := s.SaveQueue(config.Queue{Name: "Tonight", AtOnce: 2, WhenDone: config.WhenDoneShutdown})
	if err != nil {
		t.Fatal(err)
	}
	srv := httpdltest.NewServer(t)
	data := make([]byte, 64<<10)
	_, _ = rand.Read(data)
	d, _, err := s.Engine.Add(engine.AddRequest{URL: srv.Add("/a.bin", data), Queue: night.ID, Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	s.Engine.StartQueue(night.ID)
	waitUntil(t, "the countdown", func() bool { return s.PendingCountdown() != nil })
	if c := s.PendingCountdown(); c.Name != "Tonight" || c.Action != config.WhenDoneShutdown || time.Until(c.At) <= 0 {
		t.Errorf("countdown = %+v", c)
	}
	if got, _ := s.Engine.Get(d.ID); got.Status != engine.StatusDone {
		t.Errorf("the queue was done with its download %s", got.Status)
	}
	waitUntil(t, "the shutdown", func() bool { return len(acts.list()) == 1 })
	if s.PendingCountdown() != nil {
		t.Error("the countdown is still under way")
	}
	mu.Lock()
	if len(heard) != 2 || heard[0] == nil || heard[1] != nil {
		t.Errorf("heard %v, want the countdown then its end", heard)
	}
	mu.Unlock()

	// Again, cancelled this time.
	if _, _, err := s.Engine.Add(engine.AddRequest{URL: srv.Add("/b.bin", data), Queue: night.ID, Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	s.Engine.StartQueue(night.ID)
	waitUntil(t, "the second countdown", func() bool { return s.PendingCountdown() != nil })
	s.CancelCountdown()
	time.Sleep(2 * countdownTime)
	if got := acts.list(); len(got) != 1 {
		t.Errorf("actions %v: the called-off one happened", got)
	}
}

func TestDeletingAQueueMovesItsDownloadsToMain(t *testing.T) {
	s := startService(t)
	night, err := s.SaveQueue(config.Queue{Name: "Tonight", AtOnce: 1})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, name := range []string{"/1", "/2"} {
		d, _, err := s.Engine.Add(engine.AddRequest{URL: "https://example.invalid" + name, Queue: night.ID, Paused: true})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, d.ID)
	}
	if err := s.DeleteQueue(night.ID, false); err != nil {
		t.Fatal(err)
	}
	first, _ := s.Engine.Get(ids[0])
	second, _ := s.Engine.Get(ids[1])
	if first.Queue != config.MainQueue || second.Queue != config.MainQueue || first.Position > second.Position {
		t.Errorf("after deleting: %s at %d, %s at %d", first.Queue, first.Position, second.Queue, second.Position)
	}
	if _, ok := s.Config().Queue(night.ID); ok {
		t.Error("the queue is still there")
	}
	if err := s.DeleteQueue(config.MainQueue, true); err == nil {
		t.Error("Main was deleted")
	}
	if _, err := s.SaveQueue(config.Queue{Name: "  "}); err == nil {
		t.Error("a queue without a name was saved")
	}
}

func TestDeletingAQueueWithItsDownloads(t *testing.T) {
	s := startService(t)
	season, err := s.SaveQueue(config.Queue{Name: "Lanterns", AtOnce: 2})
	if err != nil {
		t.Fatal(err)
	}
	waiting, _, err := s.Engine.Add(engine.AddRequest{URL: "https://example.invalid/2", Queue: season.ID, Paused: true})
	if err != nil {
		t.Fatal(err)
	}
	kept, _, err := s.Engine.Add(engine.AddRequest{URL: "https://example.invalid/0", Queue: config.MainQueue, Paused: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteQueue(season.ID, true); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the queue's download to go", func() bool {
		_, err := s.Engine.Get(waiting.ID)
		return errors.Is(err, engine.ErrNotFound)
	})
	if _, err := s.Engine.Get(kept.ID); err != nil {
		t.Errorf("a download in Main went too: %v", err)
	}
	if _, ok := s.Config().Queue(season.ID); ok {
		t.Error("the queue is still there")
	}
}

func TestAQueuesSettingsReachTheEngine(t *testing.T) {
	s := startService(t)
	night, err := s.SaveQueue(config.Queue{Name: "Tonight", AtOnce: 1})
	if err != nil {
		t.Fatal(err)
	}
	srv := httpdltest.NewServer(t)
	srv.SetDelay(200 * time.Millisecond)
	data := make([]byte, 4<<20)
	var ds []engine.Download
	for _, name := range []string{"/1.bin", "/2.bin"} {
		d, _, err := s.Engine.Add(engine.AddRequest{URL: srv.Add(name, data), Queue: night.ID, Confirmed: true})
		if err != nil {
			t.Fatal(err)
		}
		ds = append(ds, d)
	}
	s.Engine.StartQueue(night.ID)
	waitUntil(t, "the first to run", func() bool {
		d, _ := s.Engine.Get(ds[0].ID)
		return d.Status == engine.StatusDownloading
	})
	if d, _ := s.Engine.Get(ds[1].ID); d.Status != engine.StatusQueued {
		t.Errorf("the second is %s, want waiting its turn", d.Status)
	}
	states := s.Queues()
	if len(states) != 2 || states[0].ID != config.MainQueue || !states[1].Running {
		t.Errorf("queues = %+v", states)
	}
}

func waitUntil(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}
