package subscriptions

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"slices"
	"sync"

	"github.com/Roham1299/snag/internal/store"
)

// ErrNotFound means there is no subscription with that id.
var ErrNotFound = errors.New("no such subscription")

// Store persists subscriptions to a file. It is safe for concurrent use.
type Store struct {
	file *store.File[[]Subscription]
	log  *slog.Logger

	mu       sync.Mutex
	list     []Subscription // in creation order
	onChange []func()
}

// Open loads the subscriptions at path; a missing file means none.
func Open(path string, log *slog.Logger) *Store {
	if log == nil {
		log = slog.Default()
	}
	st := &Store{file: store.NewFile[[]Subscription](path), log: log}
	if err := st.file.Load(&st.list); err != nil {
		log.Warn("couldn't read the subscriptions", "err", err)
	}
	for i := range st.list {
		st.list[i].EveryHours = clampHours(st.list[i].EveryHours)
	}
	return st
}

// OnChange calls f after every change.
func (st *Store) OnChange(f func()) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.onChange = append(st.onChange, f)
}

// List returns every subscription in creation order.
func (st *Store) List() []Subscription {
	st.mu.Lock()
	defer st.mu.Unlock()
	out := make([]Subscription, len(st.list))
	for i, s := range st.list {
		s.Seen = slices.Clone(s.Seen)
		out[i] = s
	}
	return out
}

// Get returns the subscription with id.
func (st *Store) Get(id string) (Subscription, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	i := st.index(id)
	if i < 0 {
		return Subscription{}, ErrNotFound
	}
	s := st.list[i]
	s.Seen = slices.Clone(s.Seen)
	return s, nil
}

// Following returns the subscription to source, if any.
func (st *Store) Following(source string) (Subscription, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, s := range st.list {
		if s.Source == source {
			return s, true
		}
	}
	return Subscription{}, false
}

// Add stores s, or for a list already followed updates the existing subscription with s's
// choices and seen entries, and returns what was stored.
func (st *Store) Add(s Subscription) (Subscription, error) {
	st.mu.Lock()
	for i := range st.list {
		if old := &st.list[i]; old.Source == s.Source {
			old.Quality, old.Directory, old.Queue, old.Naming = s.Quality, s.Directory, s.Queue, s.Naming
			old.EveryHours, old.Paused = s.EveryHours, false
			old.see(s.Seen)
			kept := *old
			return kept, st.saveAndTell()
		}
	}
	s.ID = newID()
	s.EveryHours = clampHours(s.EveryHours)
	st.list = append(st.list, s)
	return s, st.saveAndTell()
}

// Update applies change to the subscription with id and returns the result.
func (st *Store) Update(id string, change func(*Subscription)) (Subscription, error) {
	st.mu.Lock()
	i := st.index(id)
	if i < 0 {
		st.mu.Unlock()
		return Subscription{}, ErrNotFound
	}
	change(&st.list[i])
	st.list[i].EveryHours = clampHours(st.list[i].EveryHours)
	s := st.list[i]
	return s, st.saveAndTell()
}

// Remove deletes the subscription with id; its downloads are kept.
func (st *Store) Remove(id string) error {
	st.mu.Lock()
	i := st.index(id)
	if i < 0 {
		st.mu.Unlock()
		return ErrNotFound
	}
	st.list = slices.Delete(st.list, i, i+1)
	return st.saveAndTell()
}

// index requires the lock.
func (st *Store) index(id string) int {
	return slices.IndexFunc(st.list, func(s Subscription) bool { return s.ID == id })
}

// saveAndTell saves, releases the lock, then notifies listeners. The lock must be held on entry.
func (st *Store) saveAndTell() error {
	list := slices.Clone(st.list)
	listeners := slices.Clone(st.onChange)
	st.mu.Unlock()
	err := st.file.Save(list)
	if err != nil {
		st.log.Error("couldn't save the subscriptions", "err", err)
	}
	for _, f := range listeners {
		f()
	}
	return err
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
