package service_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/playlist"
	"github.com/Roham1299/snag/internal/service"
	"github.com/Roham1299/snag/internal/subscriptions"
)

const feedHead = `<?xml version="1.0"?><rss version="2.0"><channel><title>Harbour Hour</title>`

func episode(n string) string {
	return `<item><title>Episode ` + n + `</title><enclosure url="https://cdn.example.org/` + n + `.mp3" type="audio/mpeg"/></item>`
}

func TestFollowingAPodcast(t *testing.T) {
	var mu sync.Mutex
	items := episode("2") + episode("1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(feedHead + items + `</channel></rss>`))
	}))
	defer srv.Close()

	app := startApp(t, `{"ask": false}`)
	p, err := app.ListPlaylist(context.Background(), playlist.Request{URL: srv.URL + "/show/feed.rss"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Podcast || len(p.Entries) != 2 {
		t.Fatalf("the feed = %+v", p)
	}
	dir := t.TempDir()
	added, err := app.FollowPlaylist(p.ID, playlist.Choice{Quality: "published", Directory: dir}, 6)
	if err != nil || !added.Following || added.Added != 0 {
		t.Fatalf("FollowPlaylist = %+v, %v", added, err)
	}
	subs := app.Subscriptions()
	if len(subs) != 1 || subs[0].Title != "Harbour Hour" || subs[0].Directory != dir || subs[0].Queue != added.Queue.ID {
		t.Fatalf("subscriptions = %+v", subs)
	}
	if f, ok := app.Following(srv.URL + "/show/feed.rss"); !ok || f.ID != subs[0].ID {
		t.Error("not following its address")
	}

	var news [][]string
	app.OnSubscriptionNews(func(_ subscriptions.Subscription, titles []string) {
		mu.Lock()
		defer mu.Unlock()
		news = append(news, titles)
	})
	mu.Lock()
	items = episode("4") + episode("3") + items
	mu.Unlock()
	if err := app.CheckSubscription(subs[0].ID); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool { return len(app.Subscriptions()) == 1 && app.Subscriptions()[0].Last == 2 })
	var urls []string
	for _, d := range app.Engine.List() {
		urls = append(urls, d.URL)
	}
	// Newest first in the list: the oldest new one was added first.
	if strings.Join(urls, " ") != "https://cdn.example.org/4.mp3 https://cdn.example.org/3.mp3" {
		t.Errorf("downloads = %v", urls)
	}
	mu.Lock()
	if len(news) != 1 || strings.Join(news[0], ", ") != "Episode 3, Episode 4" {
		t.Errorf("news = %v", news)
	}
	mu.Unlock()

	if err := app.CheckSubscription(subs[0].ID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	waitUntil(t, func() bool { return !app.CheckingSubscription(subs[0].ID) })
	if n := len(app.Engine.List()); n != 2 {
		t.Errorf("%d downloads after looking again", n)
	}

	changed, err := app.ChangeSubscription(subs[0].ID, serviceChange(subs[0], 12, true))
	if err != nil || changed.EveryHours != 12 || !changed.Paused {
		t.Errorf("ChangeSubscription = %+v, %v", changed, err)
	}
	if _, err := app.ChangeSubscription(subs[0].ID, serviceChangeQuality(subs[0], "nope")); err == nil {
		t.Error("took an unknown quality")
	}
	if err := app.Unsubscribe(subs[0].ID); err != nil || len(app.Subscriptions()) != 0 {
		t.Errorf("Unsubscribe: %v", err)
	}
	if n := len(app.Engine.List()); n != 2 {
		t.Errorf("unsubscribing removed downloads: %d left", n)
	}
}

func waitUntil(t *testing.T, done func() bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if done() {
			return
		}
	}
	t.Fatal("timed out")
}

func serviceChange(s subscriptions.Subscription, every int, paused bool) service.SubscriptionChange {
	return service.SubscriptionChange{
		Title: s.Title, Quality: s.Quality, Directory: s.Directory, Queue: s.Queue, Naming: s.Naming,
		EveryHours: every, Paused: paused, Filters: s.Filters,
	}
}

func serviceChangeQuality(s subscriptions.Subscription, quality string) service.SubscriptionChange {
	c := serviceChange(s, s.EveryHours, s.Paused)
	c.Quality = quality
	return c
}

// A podcast found via the feed its site links to is followed by that feed, though the site's
// address doesn't look like one.
func TestFollowingAPodcastFromItsSite(t *testing.T) {
	var mu sync.Mutex
	items := episode("1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path != "/episodes" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(feedHead + items + `</channel></rss>`))
	}))
	defer srv.Close()

	app := startApp(t, `{"ask": false}`)
	p, err := app.ListPlaylist(context.Background(), playlist.Request{
		URL: srv.URL + "/show", Feeds: []string{srv.URL + "/episodes"},
	})
	if err != nil || !p.Podcast || p.Feed != srv.URL+"/episodes" {
		t.Fatalf("ListPlaylist = %+v, %v", p, err)
	}
	// The episode ticked now is the subscription's too.
	added, err := app.FollowPlaylist(p.ID, playlist.Choice{Entries: []int{0}, Quality: "published", Directory: t.TempDir()}, 6)
	if err != nil || added.Added != 1 {
		t.Fatalf("FollowPlaylist = %+v, %v", added, err)
	}
	mu.Lock()
	items = episode("2") + items
	mu.Unlock()
	sub := app.Subscriptions()[0]
	if err := app.CheckSubscription(sub.ID); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool { s := app.Subscriptions(); return len(s) == 1 && s[0].Last == 1 })
	if s := app.Subscriptions()[0]; s.Error != "" {
		t.Errorf("the check failed: %s", s.Error)
	}
	l := app.Engine.List()
	if len(l) != 2 || l[0].URL != "https://cdn.example.org/2.mp3" {
		t.Fatalf("downloads = %+v", l)
	}
	for _, d := range l {
		if d.Subscription != sub.ID {
			t.Errorf("%s is from subscription %q, not %q", d.URL, d.Subscription, sub.ID)
		}
	}
}
