package playlist

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/meta"
)

// tmdbCopy serves a two-season show; a non-empty key is required like TMDB requires it.
func tmdbCopy(t *testing.T, key string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != key {
			http.Error(w, "no key", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/3/tv/95350":
			_, _ = w.Write([]byte(`{"id": 95350, "name": "Lanterns", "seasons": [
				{"season_number": 0, "name": "Specials"}, {"season_number": 1}, {"season_number": 2}]}`))
		case "/3/tv/95350/season/1":
			_, _ = w.Write([]byte(`{"episodes": [
				{"episode_number": 1, "name": "Pilot", "runtime": 57, "air_date": "2026-08-16"},
				{"episode_number": 2, "name": "Trust Fall", "runtime": 52, "air_date": "2026-08-23"},
				{"episode_number": 3, "name": "OutKast", "runtime": 54, "air_date": "2026-08-30"}]}`))
		case "/3/tv/95350/season/2":
			_, _ = w.Write([]byte(`{"episodes": [
				{"episode_number": 1, "name": "", "air_date": "2026-09-01"},
				{"episode_number": 2, "name": "Not yet", "air_date": "2099-01-01"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

var october = func() time.Time { return time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC) }

func TestEpisodesFromThePagesCopy(t *testing.T) {
	srv := tmdbCopy(t, "")
	l := Episodes{Now: october}
	p, err := l.List(context.Background(), Request{URL: "https://www.streamer.example/tv/95350/1/2?play=true#t=40", TMDB: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, e := range p.Entries {
		titles = append(titles, e.Title)
	}
	if want := []string{"S1 E1 · Pilot", "S1 E2 · Trust Fall", "S1 E3 · OutKast", "S2 E1"}; !slices.Equal(titles, want) {
		t.Errorf("titles %q, want %q", titles, want)
	}
	if p.Title != "Lanterns" || p.Current != 1 || p.Via != ViaBrowser || p.Site != "www.streamer.example" {
		t.Errorf("list %+v", p)
	}
	second := p.Entries[1]
	if second.URL != "https://www.streamer.example/tv/95350/1/2?play=true" || second.Duration != 52*60 {
		t.Errorf("second episode %+v", second)
	}
	if p.Entries[0].Group != "Season 1" || p.Entries[3].Group != "Season 2" {
		t.Errorf("seasons %q, %q", p.Entries[0].Group, p.Entries[3].Group)
	}
	if p.Entries[3].Meta.Tags.Title != "Episode 1" || p.Entries[3].Meta.Tags.Show != "Lanterns" {
		t.Errorf("an untitled episode is tagged %+v", p.Entries[3].Meta.Tags)
	}
	want := meta.Label{Only: true, Tags: meta.Tags{Title: "Trust Fall", Show: "Lanterns", Season: 1, Episode: 2,
		Date: "2026-08-23", Page: "https://www.streamer.example/tv/95350/1/2?play=true"}}
	if second.Meta != want {
		t.Errorf("second episode labelled %+v, want %+v", second.Meta, want)
	}
	if title := p.Entries[3].Meta.Tags.Title; title != "Episode 1" {
		t.Errorf("an untitled episode is tagged %q", title)
	}
}

func TestEpisodesKeepThePagesQuery(t *testing.T) {
	srv := tmdbCopy(t, "")
	p, err := Episodes{Now: october}.List(context.Background(), Request{URL: "https://other.example/watch/tv/95350?server=2", TMDB: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Entries[0].URL; got != "https://other.example/watch/tv/95350/1/1?server=2" {
		t.Errorf("first episode at %s", got)
	}
	if p.Current != -1 {
		t.Errorf("the show's page plays entry %d", p.Current)
	}
}

func TestEpisodesWithTheUsersKey(t *testing.T) {
	official := tmdbCopy(t, "k3y")
	l := Episodes{Key: func() string { return " k3y " }, TMDB: official.URL, Now: october}
	p, err := l.List(context.Background(), Request{URL: "https://site.example/tv/95350/1/3", Title: "Lanterns S1 E3 - Site"})
	if err != nil || len(p.Entries) != 4 || p.Current != 2 {
		t.Fatalf("listed %+v, %v", p, err)
	}
	if _, err := l.List(context.Background(), Request{URL: "https://site.example/tv/95350", Title: "Something else"}); !errors.Is(err, ErrNotAList) {
		t.Errorf("a page about another show: %v", err)
	}
	// The user's key must never be sent to a site's mirror.
	site := tmdbCopy(t, "")
	for _, c := range l.clients(site.URL) {
		if c.Key != "" && c.Base == site.URL {
			t.Error("the user's key went to a site's copy")
		}
	}
}

func TestEpisodesLeaveOtherPages(t *testing.T) {
	srv := tmdbCopy(t, "")
	l := Episodes{Now: october}
	for _, req := range []Request{
		{URL: "https://www.youtube.com/watch?v=x&list=PLx", TMDB: srv.URL},
		{URL: "https://www.streamer.example/tv/95350"}, // no mirror and no key
		{URL: "https://www.streamer.example/movie/12", TMDB: srv.URL},
	} {
		if _, err := l.List(context.Background(), req); !errors.Is(err, ErrNotAList) {
			t.Errorf("%s: %v", req.URL, err)
		}
	}
}

func TestSameShow(t *testing.T) {
	for _, c := range []struct {
		name, title string
		want        bool
	}{
		{"Lanterns", "Lanterns | Streamer", true},
		{"The Bear", "Watch The Bear S2 online", true},
		{"Lost", "Lost in Space", true},
		{"Lanterns", "Green Lantern", false},
		{"", "anything", false},
	} {
		if got := sameShow(c.name, c.title); got != c.want {
			t.Errorf("sameShow(%q, %q) = %v", c.name, c.title, got)
		}
	}
}

func TestListers(t *testing.T) {
	none := lister(func(Request) (Playlist, error) { return Playlist{}, ErrNotAList })
	broken := lister(func(Request) (Playlist, error) { return Playlist{}, errors.New("down") })
	one := lister(func(Request) (Playlist, error) { return Playlist{Title: "one"}, nil })
	if p, err := (Listers{none, one, broken}).List(context.Background(), Request{}); err != nil || p.Title != "one" {
		t.Errorf("got %+v, %v", p, err)
	}
	if _, err := (Listers{none, broken, one}).List(context.Background(), Request{}); err == nil {
		t.Error("a lister's failure was passed over")
	}
	if _, err := (Listers{none}).List(context.Background(), Request{}); !errors.Is(err, ErrNotAList) {
		t.Errorf("no list: %v", err)
	}
}

type lister func(Request) (Playlist, error)

func (l lister) List(_ context.Context, req Request) (Playlist, error) { return l(req) }
