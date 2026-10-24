package playlist

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const (
	keepLists = 20
	keepFor   = time.Hour
)

// Store keeps recently found lists by id so the user can choose from them.
type Store struct {
	mu    sync.Mutex
	lists []stored // oldest first
	now   func() time.Time
}

type stored struct {
	list Playlist
	at   time.Time
}

// Put stores p under a new id and returns it with that id.
func (s *Store) Put(p Playlist) Playlist {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.forget()
	p.ID = newID()
	s.lists = append(s.lists, stored{list: p, at: s.clock()})
	if len(s.lists) > keepLists {
		s.lists = s.lists[len(s.lists)-keepLists:]
	}
	return p
}

// Get returns the list with id.
func (s *Store) Get(id string) (Playlist, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.forget()
	for _, st := range s.lists {
		if st.list.ID == id {
			return st.list, true
		}
	}
	return Playlist{}, false
}

// forget drops expired lists. The lock must be held.
func (s *Store) forget() {
	now := s.clock()
	for len(s.lists) > 0 && now.Sub(s.lists[0].at) > keepFor {
		s.lists = s.lists[1:]
	}
}

func (s *Store) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
