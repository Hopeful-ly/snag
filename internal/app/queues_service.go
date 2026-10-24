package app

import (
	"github.com/Roham1299/snag/internal/config"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/service"
)

// QueuesService backs the download queues UI.
type QueuesService struct {
	app *App
}

// Queue is a queue's settings.
type Queue = config.Queue

// QueueState is a queue and whether it runs.
type QueueState = service.QueueState

// Countdown is a finished queue's pending "when done" action.
type Countdown = service.Countdown

// List returns the queues, Main first.
func (s *QueuesService) List() ([]QueueState, error) {
	svc, err := s.app.service()
	if err != nil {
		return nil, err
	}
	return svc.Queues(), nil
}

// Save adds a queue (one without an id) or changes one, and returns it as saved.
func (s *QueuesService) Save(q Queue) (Queue, error) {
	svc, err := s.app.service()
	if err != nil {
		return Queue{}, err
	}
	return svc.SaveQueue(q)
}

// Delete deletes a queue, with its unfinished downloads or moving them to Main.
func (s *QueuesService) Delete(id string, downloads bool) error {
	svc, err := s.app.service()
	if err != nil {
		return err
	}
	return svc.DeleteQueue(id, downloads)
}

// Arrange puts the queues in the order of ids; Main stays first.
func (s *QueuesService) Arrange(ids []string) error {
	svc, err := s.app.service()
	if err != nil {
		return err
	}
	return svc.ArrangeQueues(ids)
}

// Start starts a queue.
func (s *QueuesService) Start(id string) error {
	return s.withEngine(func(e *engine.Engine) error { e.StartQueue(id); return nil })
}

// Stop stops a queue; its running downloads go back to waiting.
func (s *QueuesService) Stop(id string) error {
	return s.withEngine(func(e *engine.Engine) error { e.StopQueue(id); return nil })
}

// Move puts downloads at the end of a queue's line.
func (s *QueuesService) Move(ids []string, queue string) error {
	return s.withEngine(func(e *engine.Engine) error { return e.MoveToQueue(ids, queue) })
}

// Reorder puts a queue's downloads in the order of ids.
func (s *QueuesService) Reorder(queue string, ids []string) error {
	return s.withEngine(func(e *engine.Engine) error { return e.Reorder(queue, ids) })
}

// Reverse reverses a queue's order.
func (s *QueuesService) Reverse(id string) error {
	return s.withEngine(func(e *engine.Engine) error { e.Reverse(id); return nil })
}

// ToFront puts a download first in its queue's line.
func (s *QueuesService) ToFront(id string) error {
	return s.withEngine(func(e *engine.Engine) error { return e.ToFront(id) })
}

// StartNow runs a download now, even if its queue is stopped or full.
func (s *QueuesService) StartNow(id string) error {
	return s.withEngine(func(e *engine.Engine) error { return e.StartNow(id) })
}

// Countdown returns the pending countdown, or nil.
func (s *QueuesService) Countdown() (*Countdown, error) {
	svc, err := s.app.service()
	if err != nil {
		return nil, err
	}
	return svc.PendingCountdown(), nil
}

// CancelCountdown cancels a finished queue's action.
func (s *QueuesService) CancelCountdown() error {
	svc, err := s.app.service()
	if err != nil {
		return err
	}
	svc.CancelCountdown()
	return nil
}

// ActNow does a finished queue's action without waiting.
func (s *QueuesService) ActNow() error {
	svc, err := s.app.service()
	if err != nil {
		return err
	}
	svc.ActNow()
	return nil
}

func (s *QueuesService) withEngine(f func(*engine.Engine) error) error {
	svc, err := s.app.service()
	if err != nil {
		return err
	}
	return f(svc.Engine)
}
