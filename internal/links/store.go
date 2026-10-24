package links

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const (
	keepSets = 20
	keepFor  = time.Hour
)

// Store keeps recent sets by id, for at most an hour.
type Store struct {
	mu   sync.Mutex
	sets []stored // oldest first
	now  func() time.Time
}

type stored struct {
	set Set
	at  time.Time
}

// Put stores s under a new id and returns it with that id.
func (st *Store) Put(s Set) Set {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.forget()
	s.ID = newID()
	st.sets = append(st.sets, stored{set: s, at: st.clock()})
	if len(st.sets) > keepSets {
		st.sets = st.sets[len(st.sets)-keepSets:]
	}
	return s
}

// Get returns the set with id.
func (st *Store) Get(id string) (Set, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.forget()
	for _, s := range st.sets {
		if s.set.ID == id {
			return s.set, true
		}
	}
	return Set{}, false
}

// Learned records what a link's server said, regrouping the link by the server's file name.
func (st *Store) Learned(id string, index int, size int64, server string, page bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	for i := range st.sets {
		if s := &st.sets[i].set; s.ID == id && index >= 0 && index < len(s.Links) {
			l := &s.Links[index]
			l.Looked = true
			if size > 0 {
				l.Size = size
			}
			if server != "" {
				l.Server = server
			}
			switch {
			case page && l.Group != VideoPages:
				l.Group = Pages
			case server != "" && l.Group != VideoPages:
				if g := Classify(l.URL, server); g != Pages {
					l.Group = g
				}
			case l.Group == Pages:
				l.Group = Other // not a page after all
			}
		}
	}
}

// forget drops expired sets. The lock must be held.
func (st *Store) forget() {
	now := st.clock()
	for len(st.sets) > 0 && now.Sub(st.sets[0].at) > keepFor {
		st.sets = st.sets[1:]
	}
}

func (st *Store) clock() time.Time {
	if st.now != nil {
		return st.now()
	}
	return time.Now()
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
