package engine

import (
	"cmp"
	"errors"
	"slices"
	"time"

	"github.com/Roham1299/snag/internal/naming"
	"github.com/Roham1299/snag/internal/ratelimit"
)

// ErrNoRunner means Snag has no way to download that kind of address.
var ErrNoRunner = errors.New("this kind of address can't be downloaded yet")

// MainQueue is the default queue; it always runs unless stopped. Other queues run once started
// and stop when done.
const MainQueue = "main"

// QueueSettings are a queue's settings the engine follows.
type QueueSettings struct {
	ID     string
	AtOnce int
	// SpeedLimit caps its downloads together, in bytes a second; 0 for none.
	SpeedLimit int64
	// Retries is how many times a failed download is tried again before the next one starts.
	Retries   int
	RetryWait time.Duration
	// Fallback is where a song is looked for when YouTube Music has no match ("youtube" or
	// "soundcloud"); "" asks the user.
	Fallback string
}

// QueueEventKind says what happened to a queue.
type QueueEventKind string

// The kinds of queue event.
const (
	QueueStarted QueueEventKind = "started"
	QueueStopped QueueEventKind = "stopped"
	// QueueDone means a queue finished everything since it started; queues other than Main then stop.
	QueueDone QueueEventKind = "done"
)

// QueueEvent is a change to a queue.
type QueueEvent struct {
	Kind  QueueEventKind
	Queue string
}

// queueOf is the queue a download runs in: Main for a queue that doesn't exist (any more).
// The lock must be held.
func (e *Engine) queueOf(d Download) string {
	if d.Queue == MainQueue || d.Queue == "" {
		return MainQueue
	}
	if slices.ContainsFunc(e.settings.Queues, func(q QueueSettings) bool { return q.ID == d.Queue }) {
		return d.Queue
	}
	return MainQueue
}

// queueSettings returns a queue's settings. Main runs MaxActive at once unless its settings say.
// The lock must be held.
func (e *Engine) queueSettings(id string) QueueSettings {
	return e.settings.queue(id)
}

// queue is a queue's settings, Main's from MaxActive when not given.
func (s Settings) queue(id string) QueueSettings {
	if i := slices.IndexFunc(s.Queues, func(q QueueSettings) bool { return q.ID == id }); i >= 0 {
		q := s.Queues[i]
		q.AtOnce = max(1, q.AtOnce)
		return q
	}
	return QueueSettings{ID: id, AtOnce: max(1, s.MaxActive)}
}

// trimToLimits requeues the downloads furthest back in line beyond a running queue's limit, if
// that limit dropped from old; downloads started by hand beyond an unchanged limit keep running.
// The lock must be held.
func (e *Engine) trimToLimits(old Settings) {
	running := map[string][]*entry{}
	for _, en := range e.entries {
		if en.d.Status == StatusDownloading && en.cancel != nil {
			queue := e.queueOf(en.d)
			running[queue] = append(running[queue], en)
		}
	}
	for queue, list := range running {
		limit := e.queueSettings(queue).AtOnce
		if !e.running[queue] || len(list) <= limit || limit >= old.queue(queue).AtOnce {
			continue
		}
		slices.SortStableFunc(list, func(a, b *entry) int { return cmp.Compare(a.d.Position, b.d.Position) })
		for _, en := range list[limit:] {
			e.requeue(en)
		}
	}
}

// limiter is the speed limit a queue's downloads share. The lock must be held.
func (e *Engine) limiter(q QueueSettings) *ratelimit.Limiter {
	l := e.limiters[q.ID]
	if l == nil {
		l = ratelimit.New(q.SpeedLimit)
		e.limiters[q.ID] = l
	}
	return l
}

// nextPosition is a place at the end of every line. The lock must be held.
func (e *Engine) nextPosition() int64 {
	e.lastPosition++
	return e.lastPosition
}

// QueueRunning reports whether a queue runs now.
func (e *Engine) QueueRunning(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running[id]
}

// StartQueue starts a queue running its downloads in order.
func (e *Engine) StartQueue(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running[id] || e.closed {
		return
	}
	e.running[id] = true
	e.queueEvent(QueueStarted, id)
	e.schedule()
}

// StopQueue stops a queue; its running downloads go back in line to continue when it restarts.
func (e *Engine) StopQueue(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running[id] {
		return
	}
	e.running[id], e.worked[id] = false, false
	for _, en := range e.entries {
		if e.queueOf(en.d) == id {
			e.requeue(en)
		}
	}
	e.queueEvent(QueueStopped, id)
}

// requeue stops a running download so it waits in line again. The lock must be held.
func (e *Engine) requeue(en *entry) {
	if en.retry != nil {
		e.cancelRetry(en)
		en.d.Status, en.d.Note = StatusQueued, ""
		e.markDirty()
		e.changed(en, true)
		return
	}
	if en.d.Status != StatusDownloading || en.cancel == nil {
		return
	}
	en.requeue = true
	en.d.Status = StatusPausing
	en.cancel()
	e.markDirty()
	e.changed(en, true)
}

// MoveToQueue puts downloads at the end of another queue's line. One running in a queue that
// doesn't run stops and waits there.
func (e *Engine) MoveToQueue(ids []string, queue string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, id := range ids {
		en := e.find(id)
		if en == nil {
			return ErrNotFound
		}
		if en.d.Queue == queue {
			continue
		}
		en.d.Queue, en.d.Position = queue, e.nextPosition()
		if !e.running[e.queueOf(en.d)] {
			e.requeue(en)
		}
		e.markDirty()
		e.changed(en, true)
	}
	e.schedule()
	return nil
}

// Reorder puts downloads of a queue in the order of ids, in the places they held between them:
// the rest of the line stays where it was.
func (e *Engine) Reorder(queue string, ids []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	var moved []*entry
	var places []int64
	for _, id := range ids {
		en := e.find(id)
		if en == nil {
			return ErrNotFound
		}
		if e.queueOf(en.d) != queue || slices.Contains(moved, en) {
			continue
		}
		moved = append(moved, en)
		places = append(places, en.d.Position)
	}
	slices.Sort(places)
	for i, en := range moved {
		if en.d.Position != places[i] {
			en.d.Position = places[i]
			e.markDirty()
			e.changed(en, true)
		}
	}
	e.schedule()
	return nil
}

// Reverse flips a queue's line, so the last download is next and the first waits until the end.
// One already running keeps going; the rest follow the new order.
func (e *Engine) Reverse(queue string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	var line []*entry
	for _, en := range e.entries {
		if e.queueOf(en.d) != queue || !inLine(en.d.Status) {
			continue
		}
		line = append(line, en)
	}
	if len(line) < 2 {
		return
	}
	slices.SortStableFunc(line, func(a, b *entry) int {
		return cmp.Compare(a.d.Position, b.d.Position)
	})
	places := make([]int64, len(line))
	for i, en := range line {
		places[i] = en.d.Position
	}
	changed := false
	for i, en := range line {
		if place := places[len(places)-1-i]; en.d.Position != place {
			en.d.Position = place
			changed = true
		}
	}
	if changed {
		e.markDirty()
		e.emitReset()
	}
	e.schedule()
}

// inLine reports whether a download has a place in its queue's line. Finished ones stay listed
// without taking a turn.
func inLine(s Status) bool {
	return s != StatusDone && s != StatusPending && s != StatusRemoving
}

// ToFront puts a download first in its queue's line.
func (e *Engine) ToFront(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	en := e.find(id)
	if en == nil {
		return ErrNotFound
	}
	queue := e.queueOf(en.d)
	first := en.d.Position
	for _, other := range e.entries {
		if other != en && e.queueOf(other.d) == queue {
			first = min(first, other.d.Position-1)
		}
	}
	if first != en.d.Position {
		en.d.Position = first
		e.markDirty()
		e.changed(en, true)
		e.schedule()
	}
	return nil
}

// StartNow runs a download now, whether or not its queue runs or has room.
func (e *Engine) StartNow(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	en := e.find(id)
	if en == nil {
		return ErrNotFound
	}
	switch en.d.Status {
	case StatusQueued, StatusPaused, StatusError:
	default:
		return nil
	}
	if en.cancel != nil || e.closed {
		return nil // a pausing run still has to stop; Resume handles that
	}
	if e.runners[en.d.Kind] == nil {
		return ErrNoRunner
	}
	e.start(en)
	return nil
}

// waitingInOrder returns the downloads waiting to run, first in line first. The lock must be
// held.
func (e *Engine) waitingInOrder() []*entry {
	var waiting []*entry
	for _, en := range e.entries {
		if en.d.Status == StatusQueued && en.cancel == nil {
			waiting = append(waiting, en)
		}
	}
	slices.SortStableFunc(waiting, func(a, b *entry) int {
		return cmp.Or(cmp.Compare(a.d.Position, b.d.Position), a.d.Created.Compare(b.d.Created))
	})
	return waiting
}

// checkDone ends the run of each queue that has done everything it had to: it is done, and
// stops unless it is Main. The lock must be held.
func (e *Engine) checkDone() {
	busy := map[string]bool{}
	for _, en := range e.entries {
		switch en.d.Status {
		case StatusQueued, StatusDownloading, StatusPausing:
			busy[e.queueOf(en.d)] = true
		}
	}
	for queue, worked := range e.worked {
		if !worked || busy[queue] || !e.running[queue] {
			continue
		}
		e.worked[queue] = false
		if queue != MainQueue {
			e.running[queue] = false
			e.queueEvent(QueueStopped, queue)
		}
		e.queueEvent(QueueDone, queue)
	}
}

// queueEvent tells the caller about a queue, in order and outside the lock. The lock must be
// held.
func (e *Engine) queueEvent(kind QueueEventKind, queue string) {
	if e.queueEvents == nil || e.closed {
		return
	}
	select {
	case e.queueEvents <- QueueEvent{Kind: kind, Queue: queue}:
	default:
		e.log.Warn("a queue event was dropped", "queue", queue, "kind", kind)
	}
}

// deliverQueueEvents hands queue events to onQueue, one at a time.
func (e *Engine) deliverQueueEvents(onQueue func(QueueEvent)) {
	for ev := range e.queueEvents {
		onQueue(ev)
	}
}

// RenameQueue makes queue's unfinished downloads take their names from t once each finishes;
// until then they show a name guessed from their list's details.
func (e *Engine) RenameQueue(queue string, t naming.Template) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, en := range e.entries {
		d := &en.d
		if e.queueOf(*d) != queue || d.Status == StatusDone || !d.Naming.IsSet() || d.Naming.Template == t {
			continue
		}
		d.Naming.Template = t
		if name := d.Naming.Name(d.Meta.Known(d.Title)); name != "" {
			d.Filename = name
		}
		e.markDirty()
		e.changed(en, true)
	}
}
