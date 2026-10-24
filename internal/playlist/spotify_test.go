package playlist

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Roham1299/snag/internal/meta"
	"github.com/Roham1299/snag/internal/spotify"
)

func spotifyServer(t *testing.T) spotify.Client {
	t.Helper()
	files := map[string]string{
		"/embed/playlist/37i9dQZF1DXcBWIGoYBM5M": "embed-playlist.html",
		"/embed/album/4m2880jivSbbyEGAKfITCa":    "embed-album.html",
		"/embed/track/3zOYmiNOuyDL0ClxJcEJ0T":    "embed-track.html",
		"/embed/artist/4tZwfgrHOc3mvqYlEYSvVi":   "embed-artist.html",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file := files[r.URL.Path]
		if r.URL.Path == "/pathfinder/v2/query" {
			file = "web-playlist.json"
		}
		if file == "" {
			http.NotFound(w, r)
			return
		}
		b, err := os.ReadFile("../spotify/testdata/" + file)
		if err != nil {
			t.Error(err)
		}
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return spotify.Client{HTTP: srv.Client(), Open: srv.URL, API: srv.URL}
}

func TestSpotifyPlaylistFromItsPage(t *testing.T) {
	l := Spotify{Client: spotifyServer(t)}
	session := &spotify.Session{Token: "Bearer t", Hashes: map[string]string{"fetchPlaylist": "h"}}
	p, err := l.List(context.Background(), Request{URL: "https://open.spotify.com/playlist/37i9dQZF1DX4o1oenSJRJd?si=1", Spotify: session})
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "All Out 2000s" || p.Via != ViaSearch || p.Site != "Spotify" || p.Current != -1 || p.Note != "" ||
		p.Page != "https://open.spotify.com/playlist/37i9dQZF1DX4o1oenSJRJd" || len(p.Entries) != 2 {
		t.Fatalf("list %+v", p)
	}
	first := p.Entries[0]
	want := meta.Label{
		Tags: meta.Tags{Title: "Behind Blue Eyes", Artist: "Limp Bizkit", Album: "Results May Vary",
			AlbumArtist: "Limp Bizkit", Track: 16, Disc: 1, Date: "2003-01-01",
			Cover: "https://i.scdn.co/image/ab67616d0000b27344971d48c21dbd132f2d506e",
			Page:  "https://open.spotify.com/track/1MTQHCpraD4S8g5PAFKzoj"},
		Defaults: meta.Tags{Album: "All Out 2000s", Track: 1},
	}
	if first.URL != want.Tags.Page || first.Title != "Limp Bizkit - Behind Blue Eyes" || first.Meta != want ||
		int(first.Duration) != 269 {
		t.Errorf("first entry %+v\nwant label %+v", first, want)
	}

	reqs, err := Plan(p, Choice{Entries: []int{0}, Quality: "mp3", Directory: "/music", Naming: "{artist} - {title}"})
	if err != nil {
		t.Fatal(err)
	}
	if r := reqs[0]; !r.Resolve || r.Audio != "mp3" || r.Meta != want || r.Filename != "Limp Bizkit - Behind Blue Eyes" {
		t.Errorf("planned %+v", r)
	}
}

func TestSpotifyPlaylistFromTheEmbed(t *testing.T) {
	p, err := Spotify{Client: spotifyServer(t)}.List(context.Background(),
		Request{URL: "https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Today’s Top Hits" || len(p.Entries) != 3 || p.Entries[0].Meta.Tags.Album != "" ||
		p.Entries[0].Meta.Defaults.Album != "Today’s Top Hits" {
		t.Errorf("list %+v", p)
	}
}

func TestSpotifyAlbumAndSong(t *testing.T) {
	l := Spotify{Client: spotifyServer(t)}
	album, err := l.List(context.Background(), Request{URL: "https://open.spotify.com/album/4m2880jivSbbyEGAKfITCa"})
	if err != nil {
		t.Fatal(err)
	}
	if e := album.Entries[1]; e.Title != "Get Lucky (feat. Pharrell Williams and Nile Rodgers)" ||
		e.Meta.Tags.Album != "Random Access Memories" || e.Meta.Tags.AlbumArtist != "Daft Punk" ||
		e.Meta.Tags.Artist != "Daft Punk, Pharrell Williams, Nile Rodgers" {
		t.Errorf("album entry %+v", e)
	}
	reqs, err := Plan(album, Choice{Entries: []int{1}, Quality: "mp3", Directory: "/music", Naming: "{artist} - {title}"})
	if err != nil {
		t.Fatal(err)
	}
	if reqs[0].Filename != "Daft Punk - Get Lucky (feat. Pharrell Williams and Nile Rodgers)" ||
		reqs[0].Meta.Tags.Artist != "Daft Punk, Pharrell Williams, Nile Rodgers" {
		t.Errorf("planned %+v", reqs[0])
	}

	song, err := l.List(context.Background(), Request{URL: "https://open.spotify.com/track/3zOYmiNOuyDL0ClxJcEJ0T"})
	if err != nil {
		t.Fatal(err)
	}
	if song.Title != "Ashe - Stop The Wedding!" || len(song.Entries) != 1 || song.Entries[0].Meta.Defaults.IsSet() ||
		song.Entries[0].Meta.Tags.Date != "2026-08-07" {
		t.Errorf("song %+v", song)
	}
	if _, err := l.List(context.Background(), Request{URL: "https://open.spotify.com/track/0000000000000000000000"}); !errors.Is(err, spotify.ErrNotFound) {
		t.Errorf("a missing song: %v", err)
	}
	if _, err := l.List(context.Background(), Request{URL: "https://www.youtube.com/playlist?list=x"}); !errors.Is(err, ErrNotAList) {
		t.Errorf("another site: %v", err)
	}
}

func TestSpotifyLikedSongsAndArtist(t *testing.T) {
	l := Spotify{Client: spotifyServer(t)}
	session := &spotify.Session{Token: "Bearer t", Hashes: map[string]string{"fetchPlaylist": "h"}}
	liked, err := l.List(context.Background(), Request{URL: "https://open.spotify.com/collection/tracks", Spotify: session})
	if err != nil {
		t.Fatal(err)
	}
	if liked.Title != "Liked Songs" || liked.Page != "https://open.spotify.com/collection/tracks" || len(liked.Entries) != 2 ||
		liked.Entries[0].Meta.Tags.Album != "Results May Vary" || liked.Via != ViaSearch {
		t.Errorf("liked %+v", liked)
	}
	if _, err := l.List(context.Background(), Request{URL: "https://open.spotify.com/collection/tracks"}); !errors.Is(err, spotify.ErrSignIn) {
		t.Errorf("pasted Liked Songs: %v", err)
	}

	artist, err := l.List(context.Background(), Request{URL: "https://open.spotify.com/artist/4tZwfgrHOc3mvqYlEYSvVi"})
	if err != nil {
		t.Fatal(err)
	}
	if e := artist.Entries[0]; artist.Title != "Daft Punk - Top songs" || len(artist.Entries) != 3 ||
		e.Title != "Daft Punk - One More Time" || e.Meta.Defaults.IsSet() || e.Meta.Tags.Artist != "Daft Punk" {
		t.Errorf("artist %+v", artist)
	}
}

func TestQualitiesForSongs(t *testing.T) {
	var ids []string
	for _, q := range QualitiesFor(Playlist{Via: ViaSearch}) {
		ids = append(ids, q.ID)
	}
	if strings.Join(ids, ",") != "m4a,mp3" || DefaultQuality(Playlist{Via: ViaSearch}) != "mp3" {
		t.Errorf("song qualities %v", ids)
	}
	if len(QualitiesFor(Playlist{})) != len(Qualities) {
		t.Error("a video list loses qualities")
	}
}
