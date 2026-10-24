package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Roham1299/snag/internal/config"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/platform"
)

// countdownTime is a var so tests can shorten it.
var countdownTime = time.Minute

// Countdown is a finished queue's pending "when done" action.
type Countdown struct {
	Queue  string    `json:"queue"`
	Name   string    `json:"name"`
	Action string    `json:"action"` // config.WhenDoneSleep, Shutdown or Quit
	At     time.Time `json:"at"`
}

type countdown struct {
	Countdown
	timer *time.Timer
}

// QueueState is a queue and whether it runs.
type QueueState struct {
	config.Queue
	Running bool `json:"running"`
}

// Queues returns the queues, Main first.
func (s *Service) Queues() []QueueState {
	var states []QueueState
	for _, q := range s.Config().Queues {
		states = append(states, QueueState{Queue: q, Running: s.Engine.QueueRunning(q.ID)})
	}
	return states
}

// SaveQueue adds a queue (one without an id) or changes one, and returns it as saved.
func (s *Service) SaveQueue(q config.Queue) (config.Queue, error) {
	q.Name = strings.TrimSpace(q.Name)
	if q.Name == "" {
		return config.Queue{}, errors.New("give the queue a name")
	}
	var before config.Queue
	if q.ID == "" {
		if len(s.Config().Queues) > config.MaxQueues {
			return config.Queue{}, fmt.Errorf("there can be %d queues besides Main; delete one first", config.MaxQueues)
		}
		q.ID = newQueueID()
	} else if old, ok := s.Config().Queue(q.ID); !ok {
		return config.Queue{}, errors.New("no such queue")
	} else {
		before = old
	}
	c, err := s.Update(func(c *config.Config) { c.SetQueue(q) })
	if err != nil {
		return config.Queue{}, err
	}
	saved, _ := c.Queue(q.ID)
	if saved.Naming != "" && saved.Naming != before.Naming {
		s.Engine.RenameQueue(saved.ID, saved.Naming)
	}
	return saved, nil
}

// DeleteQueue deletes a queue. With downloads, its unfinished downloads and their parts are
// deleted; otherwise they move to the end of Main. Finished downloads always stay, under Main.
func (s *Service) DeleteQueue(id string, downloads bool) error {
	if id == config.MainQueue {
		return errors.New("the Main queue can't be deleted")
	}
	s.Engine.StopQueue(id) // so nothing of it starts while it goes
	var move []string
	for _, d := range s.Engine.List() {
		switch {
		case d.Queue != id:
		case downloads && d.Status != engine.StatusDone:
			if err := s.Engine.Remove(d.ID, false); err != nil && !errors.Is(err, engine.ErrNotFound) {
				return err
			}
		default:
			move = append(move, d.ID)
		}
	}
	slices.Reverse(move) // oldest first, so they keep their order
	if err := s.Engine.MoveToQueue(move, config.MainQueue); err != nil {
		return err
	}
	_, err := s.Update(func(c *config.Config) { c.RemoveQueue(id) })
	return err
}

// ArrangeQueues puts the queues in the order of ids; Main stays first.
func (s *Service) ArrangeQueues(ids []string) error {
	_, err := s.Update(func(c *config.Config) { c.ArrangeQueues(ids) })
	return err
}

func newQueueID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "q" + hex.EncodeToString(b)
}

// OnQueuesChanged calls f whenever a queue starts or stops.
func (s *Service) OnQueuesChanged(f func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onQueues = append(s.onQueues, f)
}

// OnCountdown calls f when a queue's "when done" countdown starts, and with nil when it ends.
func (s *Service) OnCountdown(f func(*Countdown)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onCountdown = append(s.onCountdown, f)
}

// OnQuit sets the function a queue's "quit" action calls.
func (s *Service) OnQuit(f func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.quit = f
}

func (s *Service) queueEvent(ev engine.QueueEvent) {
	s.mu.Lock()
	listeners := slices.Clone(s.onQueues)
	s.mu.Unlock()
	for _, f := range listeners {
		f()
	}
	if ev.Kind != engine.QueueDone {
		return
	}
	if q, ok := s.Config().Queue(ev.Queue); ok && q.WhenDone != config.WhenDoneNothing {
		s.startCountdown(q)
	}
}

func (s *Service) startCountdown(q config.Queue) {
	c := Countdown{Queue: q.ID, Name: q.Name, Action: q.WhenDone, At: time.Now().Add(countdownTime)}
	s.mu.Lock()
	if s.countdown != nil {
		s.countdown.timer.Stop() // the newer one stands
	}
	pending := &countdown{Countdown: c}
	pending.timer = time.AfterFunc(countdownTime, func() { s.endCountdown(pending, true) })
	s.countdown = pending
	listeners := slices.Clone(s.onCountdown)
	s.mu.Unlock()
	s.log.Info("a queue is done", "queue", q.Name, "then", q.WhenDone)
	for _, f := range listeners {
		f(&c)
	}
}

// PendingCountdown is the countdown under way, or nil.
func (s *Service) PendingCountdown() *Countdown {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.countdown == nil {
		return nil
	}
	c := s.countdown.Countdown
	return &c
}

// CancelCountdown calls off the countdown under way.
func (s *Service) CancelCountdown() {
	s.mu.Lock()
	pending := s.countdown
	s.mu.Unlock()
	if pending != nil {
		s.endCountdown(pending, false)
	}
}

// ActNow does what the countdown under way would do, without waiting.
func (s *Service) ActNow() {
	s.mu.Lock()
	pending := s.countdown
	s.mu.Unlock()
	if pending != nil {
		s.endCountdown(pending, true)
	}
}

// endCountdown is idempotent; it runs the action only if act.
func (s *Service) endCountdown(pending *countdown, act bool) {
	s.mu.Lock()
	if s.countdown != pending {
		s.mu.Unlock()
		return // already ended, or replaced
	}
	pending.timer.Stop()
	s.countdown = nil
	listeners := slices.Clone(s.onCountdown)
	quit := s.quit
	s.mu.Unlock()
	for _, f := range listeners {
		f(nil)
	}
	if !act {
		s.log.Info("the user called off a queue's when-done action", "queue", pending.Name)
		return
	}
	if err := whenDone(pending.Action, quit); err != nil {
		s.log.Warn("couldn't do what a queue does when it is done", "action", pending.Action, "err", err)
	}
}

// whenDone is a var so tests can replace it.
var whenDone = func(action string, quit func()) error {
	switch action {
	case config.WhenDoneShutdown:
		return platform.PowerOff()
	case config.WhenDoneSleep:
		return platform.Sleep()
	case config.WhenDoneQuit:
		if quit != nil {
			quit()
		}
	}
	return nil
}
