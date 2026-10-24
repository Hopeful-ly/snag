package playlist

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Roham1299/snag/internal/applemusic"
	"github.com/Roham1299/snag/internal/meta"
)

func TestAppleMusicAlbum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/lookup" {
			http.NotFound(w, r)
			return
		}
		b, err := os.ReadFile("../applemusic/testdata/album.html")
		if err != nil {
			t.Error(err)
		}
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)
	l := AppleMusic{Client: applemusic.Client{HTTP: srv.Client(), Page: srv.URL, Lookup: srv.URL + "/lookup"}}
	p, err := l.List(context.Background(), Request{URL: "https://music.apple.com/us/album/live/1545371283"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Mehrad Hidden Online Performance in Tehran (Live)" || p.Via != ViaSearch ||
		p.Site != "Apple Music" || p.Current != -1 || p.Note != "" || len(p.Entries) != 2 {
		t.Fatalf("list %+v", p)
	}
	first := p.Entries[0]
	want := meta.Label{
		Tags: meta.Tags{Title: "Barf (Live)", Artist: "Mehrad Hidden", Album: p.Title,
			AlbumArtist: "Mehrad Hidden", Track: 1, Date: "2020",
			Cover: "https://is1-ssl.mzstatic.com/image/thumb/cover.jpg/1400x1400bb.jpg",
			Page:  "https://music.apple.com/us/album/barf-live/1545371283?i=1545371448"},
		Defaults: meta.Tags{Album: p.Title, Track: 1},
	}
	if first.Title != "Barf (Live)" || first.Meta != want {
		t.Errorf("first %+v\nwant %+v", first.Meta, want)
	}
	reqs, err := Plan(p, Choice{Entries: []int{0}, Quality: "mp3", Directory: "/music", Naming: "{artist} - {title}"})
	if err != nil {
		t.Fatal(err)
	}
	if r := reqs[0]; !r.Resolve || r.Audio != "mp3" || r.Meta != want {
		t.Errorf("planned %+v", r)
	}

	one, err := l.List(context.Background(), Request{URL: "https://music.apple.com/us/album/live/1545371283?i=1545371999"})
	if err != nil {
		t.Fatal(err)
	}
	if one.Title != "Mehrad Hidden - Bitab (Live)" || len(one.Entries) != 1 || one.Entries[0].Meta.Defaults != (meta.Tags{}) {
		t.Fatalf("one song %+v", one)
	}
}

func TestAppleMusicPlaylistShowsWhatThePageLeftOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := os.ReadFile("../applemusic/testdata/playlist.html")
		if err != nil {
			t.Error(err)
		}
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)
	p, err := (AppleMusic{Client: applemusic.Client{HTTP: srv.Client(), Page: srv.URL, Lookup: srv.URL + "/lookup"}}).
		List(context.Background(), Request{URL: "https://music.apple.com/us/playlist/todays-hits/pl.aaaaaaaaaaaa"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Note != "Apple Music shows 1 of its 4 songs on this page." || p.Entries[0].Title != "Olivia Rodrigo - expectations" ||
		p.Entries[0].Meta.Tags.Album != "expectations" || p.Entries[0].Meta.Defaults.Album != "Today's Hits" {
		t.Fatalf("%+v entry %+v", p.Note, p.Entries[0])
	}
}
