package subscriptions

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/playlist"
)

func video(id, title string) playlist.Entry {
	return playlist.Entry{URL: "https://www.youtube.com/watch?v=" + id, Title: title, Duration: 600}
}

// world fakes a changing list, the downloads added from it, and a clock.
type world struct {
	mu      sync.Mutex
	list    playlist.Playlist
	readErr error
	addErr  error
	added   [][]string // titles per Add call
	told    [][]string
	now     time.Time
}

func (w *world) checker(st *Store) *Checker {
	return &Checker{
		Store: st,
		Read: func(context.Context, Subscription) (playlist.Playlist, error) {
			w.mu.Lock()
			defer w.mu.Unlock()
			p := w.list
			p.Entries = slices.Clone(p.Entries)
			return p, w.readErr
		},
		Add: func(s Subscription, p playlist.Playlist, indexes []int) (int, error) {
			w.mu.Lock()
			defer w.mu.Unlock()
			if w.addErr != nil {
				return 0, w.addErr
			}
			var titles []string
			for _, i := range indexes {
				titles = append(titles, p.Entries[i].Title)
			}
			w.added = append(w.added, titles)
			return len(titles), nil
		},
		Told: func(s Subscription, titles []string) {
			w.mu.Lock()
			defer w.mu.Unlock()
			w.told = append(w.told, titles)
		},
		Now: func() time.Time {
			w.mu.Lock()
			defer w.mu.Unlock()
			return w.now
		},
	}
}

func (w *world) set(entries ...playlist.Entry) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.list.Entries = entries
}

func newWorld(t *testing.T) (*world, *Store, *Checker, Subscription) {
	t.Helper()
	w := &world{now: time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)}
	w.list = playlist.Playlist{Title: "Channel", Page: "https://www.youtube.com/@channel/videos", NewestFirst: true,
		Entries: []playlist.Entry{video("aaaaaaaaaaa", "Old 2"), video("bbbbbbbbbbb", "Old 1")}}
	st := Open(filepath.Join(t.TempDir(), "subscriptions.json"), nil)
	sub, err := st.Add(New(w.list, playlist.Choice{Quality: "best", Directory: "/dl", Queue: "q"}, 6, w.now))
	if err != nil {
		t.Fatal(err)
	}
	return w, st, w.checker(st), sub
}

func TestSubscribingSeesWhatIsThere(t *testing.T) {
	w, st, c, sub := newWorld(t)
	if len(sub.Seen) != 2 || sub.ID == "" || sub.Due(w.now) {
		t.Fatalf("new subscription = %+v", sub)
	}
	// DefaultEveryHours ± 10% jitter.
	if d := sub.Next.Sub(w.now); d < 5*time.Hour+24*time.Minute || d > 6*time.Hour+36*time.Minute {
		t.Errorf("next look in %v", d)
	}
	c.Check(context.Background(), sub)
	if len(w.added) != 0 {
		t.Fatalf("downloaded what was there: %v", w.added)
	}
	if got, _ := st.Get(sub.ID); got.Last != 0 || got.Checked != w.now {
		t.Errorf("after looking: %+v", got)
	}
}

func TestNewEntriesDownloadOnceOldestFirst(t *testing.T) {
	w, st, c, sub := newWorld(t)
	w.set(video("ddddddddddd", "New 2"), video("ccccccccccc", "New 1"), video("aaaaaaaaaaa", "Old 2"), video("bbbbbbbbbbb", "Old 1"))
	c.Check(context.Background(), sub)
	if !slices.Equal(w.added[0], []string{"New 1", "New 2"}) || len(w.told) != 1 {
		t.Fatalf("added %v, told %v", w.added, w.told)
	}
	got, _ := st.Get(sub.ID)
	if got.Added != 2 || got.Last != 2 || len(got.Seen) != 4 {
		t.Errorf("after: %+v", got)
	}

	// The feed writes the same video's URL differently; it must still count as seen.
	w.set(playlist.Entry{URL: "https://youtu.be/ddddddddddd", Title: "New 2"})
	c.Check(context.Background(), got)
	if len(w.added) != 1 {
		t.Fatalf("downloaded again: %v", w.added)
	}

	again := Open(st.file.Path(), nil)
	got, _ = again.Get(sub.ID)
	if len(got.Seen) != 4 || got.Added != 2 {
		t.Errorf("after a restart: %+v", got)
	}
}

func TestFiltersAndPremieres(t *testing.T) {
	w, st, c, sub := newWorld(t)
	sub, _ = st.Update(sub.ID, func(s *Subscription) {
		s.Filters = Filters{Exclude: "trailer, teaser", NoShorts: true, MinMinutes: 2}
	})
	short := playlist.Entry{URL: "https://www.youtube.com/shorts/eeeeeeeeeee", Title: "Quick one", Duration: 30}
	trailer := video("fffffffffff", "Season 2 Trailer")
	brief := video("ggggggggggg", "Brief")
	brief.Duration = 90
	premiere := video("hhhhhhhhhhh", "Premiere tonight")
	premiere.Waiting = true
	keeper := video("iiiiiiiiiii", "The real thing")
	w.set(short, trailer, brief, premiere, keeper)
	c.Check(context.Background(), sub)
	if len(w.added) != 1 || !slices.Equal(w.added[0], []string{"The real thing"}) {
		t.Fatalf("added %v", w.added)
	}

	premiere.Waiting = false
	w.set(short, trailer, brief, premiere, keeper)
	sub, _ = st.Get(sub.ID)
	c.Check(context.Background(), sub)
	if len(w.added) != 2 || !slices.Equal(w.added[1], []string{"Premiere tonight"}) {
		t.Fatalf("added %v", w.added)
	}

	include := Filters{Include: "live, session"}
	if include.Lets(video("x", "Studio")) || !include.Lets(video("x", "Live at Home")) {
		t.Error("Include doesn't work")
	}
}

func TestFailuresBackOffAndSaySoAfterThree(t *testing.T) {
	w, st, c, sub := newWorld(t)
	w.readErr = errors.New("HTTP Error 429: Too Many Requests")
	var waits []time.Duration
	for range 4 {
		sub, _ = st.Get(sub.ID)
		c.Check(context.Background(), sub)
		sub, _ = st.Get(sub.ID)
		waits = append(waits, sub.Next.Sub(w.now))
	}
	if !slices.Equal(waits, []time.Duration{time.Hour, 2 * time.Hour, 4 * time.Hour, 6 * time.Hour}) {
		t.Errorf("waits = %v", waits)
	}
	if sub.Fails != 4 || sub.Error == "" {
		t.Errorf("after four failures: %+v", sub)
	}

	w.readErr = nil
	c.Check(context.Background(), sub)
	if sub, _ = st.Get(sub.ID); sub.Fails != 0 || sub.Error != "" {
		t.Errorf("after a good look: %+v", sub)
	}

	w.readErr = ErrOffline
	c.Check(context.Background(), sub)
	if sub, _ = st.Get(sub.ID); sub.Fails != 0 || sub.Next.Sub(w.now) != offlineWait {
		t.Errorf("offline: %+v", sub)
	}
}

func TestAFailedAddIsTriedAgain(t *testing.T) {
	w, st, c, sub := newWorld(t)
	w.set(video("ccccccccccc", "New"))
	w.addErr = errors.New("disk full")
	c.Check(context.Background(), sub)
	sub, _ = st.Get(sub.ID)
	if len(sub.Seen) != 2 || sub.Fails != 1 {
		t.Fatalf("after a failed add: %+v", sub)
	}
	w.addErr = nil
	c.Check(context.Background(), sub)
	if len(w.added) != 1 {
		t.Fatalf("added %v", w.added)
	}
}

func TestTickReadsWhatIsDue(t *testing.T) {
	w, st, c, sub := newWorld(t)
	other, _ := st.Add(New(playlist.Playlist{Title: "Other", Page: "https://e.org/other"}, playlist.Choice{}, 1, w.now))
	paused, _ := st.Add(New(playlist.Playlist{Title: "Paused", Page: "https://e.org/paused"}, playlist.Choice{}, 1, w.now))
	_, _ = st.Update(paused.ID, func(s *Subscription) { s.Paused = true })
	w.set(video("ccccccccccc", "New"))

	w.mu.Lock()
	w.now = w.now.Add(2 * time.Hour) // hourly ones due, six-hourly not
	w.mu.Unlock()
	c.Tick(context.Background())
	waitFor(t, func() bool { s, _ := st.Get(other.ID); return !s.Checked.Equal(other.Checked) })
	if s, _ := st.Get(sub.ID); !s.Checked.Equal(sub.Checked) {
		t.Error("looked at one that isn't due")
	}
	if s, _ := st.Get(paused.ID); !s.Checked.Equal(paused.Checked) {
		t.Error("looked at a paused one")
	}

	if err := c.CheckNow(sub.ID); err != nil {
		t.Fatal(err)
	}
	c.Tick(context.Background())
	waitFor(t, func() bool { s, _ := st.Get(sub.ID); return s.Last == 1 })
}

func waitFor(t *testing.T, done func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if done() {
			return
		}
	}
	t.Fatal("timed out")
}

func TestStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subs.json")
	st := Open(path, nil)
	changes := 0
	st.OnChange(func() { changes++ })
	now := time.Now()
	p := playlist.Playlist{Title: "A", Page: "https://e.org/a", Entries: []playlist.Entry{{URL: "https://e.org/1"}}}
	a, _ := st.Add(New(p, playlist.Choice{Quality: "best"}, 0, now))
	if a.EveryHours != DefaultEveryHours {
		t.Errorf("every %d hours", a.EveryHours)
	}
	p.Entries = append(p.Entries, playlist.Entry{URL: "https://e.org/2"})
	again, _ := st.Add(New(p, playlist.Choice{Quality: "720"}, 999, now))
	if again.ID != a.ID || again.Quality != "720" || again.EveryHours != MaxEveryHours || len(again.Seen) != 2 || len(st.List()) != 1 {
		t.Errorf("again = %+v", again)
	}
	if f, ok := st.Following("https://e.org/a"); !ok || f.ID != a.ID {
		t.Error("not following")
	}
	if err := st.Remove(a.ID); err != nil || len(Open(path, nil).List()) != 0 {
		t.Errorf("Remove: %v", err)
	}
	if _, err := st.Update("nope", func(*Subscription) {}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Update nothing: %v", err)
	}
	if changes != 3 {
		t.Errorf("%d changes told", changes)
	}
}

func TestKey(t *testing.T) {
	for address, want := range map[string]string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=10": "yt:dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ":                     "yt:dQw4w9WgXcQ",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ":       "yt:dQw4w9WgXcQ",
		"https://cdn.e.org/ep/12.mp3?utm_source=rss&x=1":   "https://cdn.e.org/ep/12.mp3?x=1",
		"https://e.org/a#b":                                "https://e.org/a",
	} {
		if got := Key(address); got != want {
			t.Errorf("Key(%s) = %s, want %s", address, got, want)
		}
	}
}

func TestAPodcastEpisodeIsKnownByItsID(t *testing.T) {
	w, st, c, sub := newWorld(t)
	ep := playlist.Entry{URL: "https://track.example.org/a/cdn.example.org/1.mp3?t=1", Title: "One", ID: "ep-1"}
	w.set(ep)
	c.Check(context.Background(), sub)
	// Only the tracking part of the URL changed: same episode.
	ep.URL = "https://track.example.org/b/cdn.example.org/1.mp3?t=2"
	w.set(ep)
	sub, _ = st.Get(sub.ID)
	c.Check(context.Background(), sub)
	if len(w.added) != 1 {
		t.Fatalf("added %v", w.added)
	}
}
