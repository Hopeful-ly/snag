package engine

import "time"

// EventKind says what happened to a download.
type EventKind string

// The kinds of event.
const (
	EventAdded   EventKind = "added"
	EventUpdated EventKind = "updated"
	EventRemoved EventKind = "removed" // only Download.ID is meaningful
	EventReset   EventKind = "reset"   // the whole list changed; Download is empty
)

// Event is a change to the download list.
type Event struct {
	Kind     EventKind
	Download Download
}

const subscriberBuffer = 256

// Subscribe returns a channel of changes to the list and a function that ends the subscription.
// A subscriber that falls behind misses events rather than holding up downloads; List always
// has the full state.
func (e *Engine) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, subscriberBuffer)
	e.mu.Lock()
	e.subscribers[ch] = struct{}{}
	e.mu.Unlock()

	unsubscribe := func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		if _, ok := e.subscribers[ch]; ok {
			delete(e.subscribers, ch)
			close(ch)
		}
	}
	return ch, unsubscribe
}

// emit sends an event to every subscriber. The lock must be held.
func (e *Engine) emit(kind EventKind, d Download) {
	for ch := range e.subscribers {
		select {
		case ch <- Event{Kind: kind, Download: d}:
		default:
		}
	}
}

// changed tells subscribers about a change to en: always when forced (a new status, a name),
// otherwise at most every updateInterval. The lock must be held.
func (e *Engine) changed(en *entry, force bool) {
	now := time.Now()
	if !force && now.Sub(en.lastEvent) < updateInterval {
		return
	}
	en.lastEvent = now
	e.emit(EventUpdated, en.d)
}

// emitReset tells subscribers the list changed as a whole. A full channel loses one older
// event to make room: the reset carries every download, so a missed one is in it. The lock
// must be held.
func (e *Engine) emitReset() {
	ev := Event{Kind: EventReset}
	for ch := range e.subscribers {
		select {
		case ch <- ev:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- ev:
			default:
			}
		}
	}
}
