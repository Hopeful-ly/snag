package playlist

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestParseAPodcast(t *testing.T) {
	data, err := os.ReadFile("testdata/podcast.rss")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseFeed(data, "https://feeds.example.org/harbour.rss")
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "The Harbour Hour" || !p.Podcast || !p.NewestFirst || p.Feed == "" || len(p.Entries) != 3 {
		t.Fatalf("feed = %+v", p)
	}
	first, second, note := p.Entries[0], p.Entries[1], p.Entries[2]
	if first.URL != "https://cdn.example.org/harbour/12.mp3?src=rss" || first.Duration != 3723 || first.ID != "harbour-12" {
		t.Errorf("first = %+v", first)
	}
	tags := first.Meta.Tags
	if tags.Title != "Episode 12: Night Ferries" || tags.Artist != "Mira Okafor" || tags.Album != "The Harbour Hour" ||
		tags.Episode != 12 || tags.Season != 2 || tags.Date != "2026-10-14" || tags.Cover != "https://harbour.example.org/cover.jpg" {
		t.Errorf("first's tags = %+v", tags)
	}
	if second.URL != "https://feeds.example.org/harbour/11.m4a" || second.Title != "Episode 11: Low Tide & Signal Fires" ||
		second.Duration != 2723 || second.Meta.Tags.Date != "2026-10-07" || second.Meta.Tags.Cover != "https://harbour.example.org/11.jpg" {
		t.Errorf("second = %+v", second)
	}
	if KindOf(p, "published") != KindSongs {
		t.Errorf("a podcast's episodes are named as %s", KindOf(p, "published"))
	}
	if note.URL != "https://harbour.example.org/notes/1" || note.Meta.Defaults.Title == "" {
		t.Errorf("an item without a file links to its page: %+v", note)
	}
	if DefaultQuality(p) != "published" || len(QualitiesFor(p)) != 2 {
		t.Errorf("a podcast's qualities: %s, %v", DefaultQuality(p), QualitiesFor(p))
	}
	if q, ok := QualityOf("published"); !ok || q.Format != "b" || q.Audio != "" {
		t.Errorf("as published = %+v", q)
	}
}

func TestParseYouTubesFeed(t *testing.T) {
	data, err := os.ReadFile("testdata/youtube.atom")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseFeed(data, "https://www.youtube.com/feeds/videos.xml?channel_id=UCK8sQmJBp8GCxrOtXWBpyEA")
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Google" || p.Podcast || len(p.Entries) != 2 {
		t.Fatalf("feed = %+v", p)
	}
	if e := p.Entries[0]; e.URL != "https://www.youtube.com/watch?v=eSGqW_VMMXQ" || e.Meta.Defaults.Date != "2026-10-15" {
		t.Errorf("first = %+v", e)
	}
}

func TestNotAFeed(t *testing.T) {
	for _, data := range []string{"<html><body>hi</body></html>", "not xml at all", `<rss><channel><title>Empty</title></channel></rss>`} {
		if _, err := ParseFeed([]byte(data), "https://e.org/feed"); !errors.Is(err, ErrNotAList) {
			t.Errorf("ParseFeed(%q) = %v", data, err)
		}
	}
}

func TestLooksLikeFeed(t *testing.T) {
	for address, want := range map[string]bool{
		"https://feeds.example.org/harbour":           true,
		"https://example.org/podcast.rss":             true,
		"https://example.org/feed/":                   true,
		"https://example.org/blog/feed":               true,
		"https://example.org/index.xml":               true,
		"https://anchor.fm/s/12345/podcast/rss":       true,
		"https://example.org/?format=rss":             true,
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ": false,
		"https://example.org/feeding-cats":            false,
		"https://example.org/":                        false,
	} {
		if got := LooksLikeFeed(address); got != want {
			t.Errorf("LooksLikeFeed(%s) = %v", address, got)
		}
	}
}

func TestFeedLister(t *testing.T) {
	data, _ := os.ReadFile("testdata/podcast.rss")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/show/feed.rss" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write(data)
	}))
	defer srv.Close()
	f := Feed{}
	p, err := f.List(context.Background(), Request{URL: srv.URL + "/show/feed.rss"})
	if err != nil || len(p.Entries) != 3 {
		t.Fatalf("List = %+v, %v", p, err)
	}
	if _, err := f.List(context.Background(), Request{URL: srv.URL + "/watch"}); !errors.Is(err, ErrNotAList) {
		t.Errorf("an ordinary page: %v", err)
	}
	if _, err := f.List(context.Background(), Request{URL: srv.URL + "/gone/feed.rss"}); err == nil || errors.Is(err, ErrNotAList) {
		t.Errorf("a missing feed: %v", err)
	}
}

func TestFeedFromPage(t *testing.T) {
	podcast, _ := os.ReadFile("testdata/podcast.rss")
	articles, _ := os.ReadFile("testdata/youtube.atom") // not a podcast: entries are pages
	asked := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked[r.URL.Path]++
		switch r.URL.Path {
		case "/posts.atom":
			_, _ = w.Write(articles)
		case "/episodes":
			_, _ = w.Write(podcast)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	f := Feed{}
	page := srv.URL + "/show"

	p, err := f.List(context.Background(), Request{URL: page, Feeds: []string{
		srv.URL + "/posts.atom", srv.URL + "/gone", "javascript:void(0)", srv.URL + "/episodes",
	}})
	if err != nil || !p.Podcast || len(p.Entries) != 3 || p.Feed != srv.URL+"/episodes" {
		t.Fatalf("List = %+v, %v", p, err)
	}

	if _, err := f.List(context.Background(), Request{URL: page, Feeds: []string{srv.URL + "/posts.atom"}}); !errors.Is(err, ErrNotAList) {
		t.Errorf("a blog: %v", err)
	}
	clear(asked)
	if _, err := f.List(context.Background(), Request{URL: page}); !errors.Is(err, ErrNotAList) || len(asked) != 0 {
		t.Errorf("no feeds: %v, asked %v", err, asked)
	}

	if p, err := f.List(context.Background(), Request{
		URL: srv.URL + "/posts.atom", Feeds: []string{srv.URL + "/episodes"}, FeedsOnly: true,
	}); err != nil || p.Feed != srv.URL+"/episodes" {
		t.Errorf("feeds only = %+v, %v", p, err)
	}
	other := listerFunc(func(context.Context, Request) (Playlist, error) {
		return Playlist{Title: "other"}, nil
	})
	if _, err := (Listers{f, other}).List(context.Background(), Request{
		URL: page, Feeds: []string{srv.URL + "/posts.atom"}, FeedsOnly: true,
	}); !errors.Is(err, ErrNotAList) {
		t.Errorf("feeds only, none a podcast's: %v", err)
	}

	// Only maxPageFeeds are read.
	clear(asked)
	many := []string{srv.URL + "/a", srv.URL + "/b", srv.URL + "/c", srv.URL + "/episodes"}
	if _, err := f.List(context.Background(), Request{URL: page, Feeds: many}); !errors.Is(err, ErrNotAList) || asked["/episodes"] != 0 {
		t.Errorf("many feeds: %v, asked %v", err, asked)
	}
}

type listerFunc func(context.Context, Request) (Playlist, error)

func (f listerFunc) List(ctx context.Context, req Request) (Playlist, error) { return f(ctx, req) }
