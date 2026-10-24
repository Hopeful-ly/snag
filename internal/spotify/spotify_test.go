package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	for address, want := range map[string]Ref{
		"https://open.spotify.com/playlist/37i9dQZF1DX4o1oenSJRJd":             {Playlist, "37i9dQZF1DX4o1oenSJRJd"},
		"https://open.spotify.com/intl-de/album/4m2880jivSbbyEGAKfITCa?si=x":   {Album, "4m2880jivSbbyEGAKfITCa"},
		"https://open.spotify.com/embed/track/3zOYmiNOuyDL0ClxJcEJ0T":          {Track, "3zOYmiNOuyDL0ClxJcEJ0T"},
		"spotify:track:3zOYmiNOuyDL0ClxJcEJ0T":                                 {Track, "3zOYmiNOuyDL0ClxJcEJ0T"},
		"https://open.spotify.com/artist/4tZwfgrHOc3mvqYlEYSvVi":               {Artist, "4tZwfgrHOc3mvqYlEYSvVi"},
		"https://open.spotify.com/collection/tracks":                           LikedSongs,
		"https://open.spotify.com/intl-fr/collection/tracks/":                  LikedSongs,
		"spotify:collection:tracks":                                            LikedSongs,
		"spotify:user:someone:collection":                                      LikedSongs,
		"https://open.spotify.com/collection/albums":                           {},
		"https://example.com/playlist/37i9dQZF1DX4o1oenSJRJd":                  {},
		"https://open.spotify.com/playlist/short":                              {},
		"https://open.spotify.com/episode/3zOYmiNOuyDL0ClxJcEJ0T":              {},
		"https://open.spotify.com/playlist/37i9dQZF1DX4o1oenSJRJd/extra/stuff": {},
	} {
		got, ok := Parse(address)
		if got != want || ok != (want != Ref{}) {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", address, got, ok, want)
		}
	}
	if u := (Ref{Track, "3zOYmiNOuyDL0ClxJcEJ0T"}).URL(); u != "https://open.spotify.com/track/3zOYmiNOuyDL0ClxJcEJ0T" {
		t.Errorf("URL = %q", u)
	}
	if u := LikedSongs.URL(); u != "https://open.spotify.com/collection/tracks" {
		t.Errorf("Liked Songs' URL = %q", u)
	}
}

// spotify serves the recorded pages and answers, and records the queries made.
type spotify struct {
	*httptest.Server
	queries []map[string]any
	refuse  bool
}

func newSpotify(t *testing.T) *spotify {
	t.Helper()
	s := &spotify{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serve := func(file string) {
			b, err := os.ReadFile("testdata/" + file)
			if err != nil {
				t.Error(err)
			}
			_, _ = w.Write(b)
		}
		switch r.URL.Path {
		case "/embed/playlist/37i9dQZF1DXcBWIGoYBM5M":
			serve("embed-playlist.html")
		case "/embed/album/4m2880jivSbbyEGAKfITCa":
			serve("embed-album.html")
		case "/embed/track/3zOYmiNOuyDL0ClxJcEJ0T":
			serve("embed-track.html")
		case "/embed/artist/4tZwfgrHOc3mvqYlEYSvVi":
			serve("embed-artist.html")
		case "/pathfinder/v2/query":
			var q map[string]any
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &q)
			q["authorization"] = r.Header.Get("Authorization")
			s.queries = append(s.queries, q)
			if s.refuse {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			switch q["operationName"] {
			case "fetchPlaylist", "fetchPlaylistMetadata":
				serve("web-playlist.json")
			case "fetchPlaylistContents":
				serve("web-liked.json") // the songs alone
			case "fetchLibraryTracks":
				serve("web-library.json")
			case "getAlbum":
				serve("web-album.json")
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *spotify) client() Client {
	return Client{HTTP: s.Client(), Open: s.URL, API: s.URL}
}

var session = &Session{Token: "Bearer t", ClientToken: "c", AppVersion: "1", UserAgent: "UA",
	Hashes: map[string]string{"fetchPlaylist": "h1", "getAlbum": "h2"}}

func TestEmbedPlaylist(t *testing.T) {
	s := newSpotify(t)
	l, err := s.client().Read(context.Background(), Ref{Playlist, "37i9dQZF1DXcBWIGoYBM5M"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if l.Title != "Today’s Top Hits" || l.Owner != "Spotify" || len(l.Songs) != 3 || l.Partial || l.Cover == "" {
		t.Errorf("list %+v", l)
	}
	first := l.Songs[0]
	if first.ID != "3zOYmiNOuyDL0ClxJcEJ0T" || first.Title != "Stop The Wedding!" || !slices.Equal(first.Artists, []string{"Ashe"}) ||
		first.Duration != 198840*time.Millisecond || first.Album != "" || first.Number != 0 {
		t.Errorf("first song %+v", first)
	}
}

func TestEmbedAlbumAndSong(t *testing.T) {
	s := newSpotify(t)
	album, err := s.client().Read(context.Background(), Ref{Album, "4m2880jivSbbyEGAKfITCa"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	lucky := album.Songs[1]
	if album.Title != "Random Access Memories" || lucky.Album != "Random Access Memories" || lucky.Number != 2 ||
		!slices.Equal(lucky.Artists, []string{"Daft Punk", "Pharrell Williams", "Nile Rodgers"}) ||
		!slices.Equal(lucky.AlbumArtists, []string{"Daft Punk"}) || !strings.Contains(lucky.Cover, "b273") {
		t.Errorf("album %+v, song %+v", album, lucky)
	}

	song, err := s.client().Read(context.Background(), Ref{Track, "3zOYmiNOuyDL0ClxJcEJ0T"}, session)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.queries) != 0 {
		t.Error("a song alone was asked of the web player's API")
	}
	if len(song.Songs) != 1 || song.Songs[0].Date != "2026-08-07" || !strings.Contains(song.Songs[0].Cover, "b273") ||
		!slices.Equal(song.Songs[0].Artists, []string{"Ashe"}) {
		t.Errorf("song %+v", song)
	}

	if _, err := s.client().Read(context.Background(), Ref{Track, "0000000000000000000000"}, nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("a missing song: %v", err)
	}
}

func TestWebPlaylist(t *testing.T) {
	s := newSpotify(t)
	l, err := s.client().Read(context.Background(), Ref{Playlist, "37i9dQZF1DX4o1oenSJRJd"}, session)
	if err != nil {
		t.Fatal(err)
	}
	// The recorded answer has 2 songs and an episode of 150: a short page ends the reading.
	if len(s.queries) != 1 || s.queries[0]["authorization"] != "Bearer t" {
		t.Errorf("queries %+v", s.queries)
	}
	if l.Title != "All Out 2000s" || l.Owner != "Spotify" || len(l.Songs) != 2 {
		t.Fatalf("list %+v", l)
	}
	first := l.Songs[0]
	if first.Title != "Behind Blue Eyes" || first.Album != "Results May Vary" || first.Number != 16 || first.Disc != 1 ||
		first.Date != "2003-01-01" || !slices.Equal(first.AlbumArtists, []string{"Limp Bizkit"}) ||
		!strings.Contains(first.Cover, "b273") || first.Duration != 269973*time.Millisecond {
		t.Errorf("first song %+v", first)
	}
}

func TestWebAlbum(t *testing.T) {
	s := newSpotify(t)
	l, err := s.client().Read(context.Background(), Ref{Album, "4m2880jivSbbyEGAKfITCa"}, session)
	if err != nil {
		t.Fatal(err)
	}
	lucky := l.Songs[1]
	if lucky.Number != 8 || lucky.Album != "Random Access Memories" || lucky.Date != "2013-05-20" || len(lucky.Artists) != 3 {
		t.Errorf("song %+v", lucky)
	}
}

// A refused session falls back to the embeds.
func TestRefusedSessionFallsBackToTheEmbed(t *testing.T) {
	s := newSpotify(t)
	s.refuse = true
	l, err := s.client().Read(context.Background(), Ref{Album, "4m2880jivSbbyEGAKfITCa"}, session)
	if err != nil || len(s.queries) != 1 || l.Songs[1].Number != 2 {
		t.Errorf("read %+v, %v after %d queries", l, err, len(s.queries))
	}
}

// A page that didn't ask for a playlist's songs still has the query, under another name.
func TestWebPlaylistByAnotherQuerysHash(t *testing.T) {
	s := newSpotify(t)
	other := &Session{Token: "Bearer t", Hashes: map[string]string{"fetchPlaylistMetadata": "h1"}}
	l, err := s.client().Read(context.Background(), Ref{Playlist, "37i9dQZF1DX4o1oenSJRJd"}, other)
	if err != nil || len(s.queries) != 1 || l.Songs[0].Album != "Results May Vary" {
		t.Errorf("read %+v, %v after %+v", l, err, s.queries)
	}
	ext := s.queries[0]["extensions"].(map[string]any)["persistedQuery"].(map[string]any)
	if ext["sha256Hash"] != "h1" || s.queries[0]["operationName"] != "fetchPlaylistMetadata" {
		t.Errorf("asked %+v", s.queries[0])
	}
}

// A page that only asked for a playlist's songs gives no name: the embed names it.
func TestWebPlaylistBySongsAlone(t *testing.T) {
	s := newSpotify(t)
	songs := &Session{Token: "Bearer t", Hashes: map[string]string{"fetchPlaylistContents": "h3"}}
	l, err := s.client().Read(context.Background(), Ref{Playlist, "37i9dQZF1DXcBWIGoYBM5M"}, songs)
	if err != nil || l.Title != "Today’s Top Hits" || l.Owner != "Spotify" || len(l.Songs) != 2 || l.Songs[0].Album != "Results May Vary" {
		t.Errorf("read %+v, %v", l, err)
	}
	if v := s.queries[0]["variables"].(map[string]any); len(v) != 3 {
		t.Errorf("asked the songs alone with %v", v)
	}
}

func TestLikedSongs(t *testing.T) {
	s := newSpotify(t)
	liked := &Session{Token: "Bearer t", Hashes: map[string]string{"fetchPlaylistContents": "h3", "getAlbum": "h2"}}
	l, err := s.client().Read(context.Background(), LikedSongs, liked)
	if err != nil {
		t.Fatal(err)
	}
	if l.Title != "Liked Songs" || l.Ref != LikedSongs || len(l.Songs) != 2 || l.Songs[1].Album == "" {
		t.Errorf("list %+v", l)
	}
	if v := s.queries[0]["variables"].(map[string]any); v["uri"] != "spotify:playlist:37i9dQZF1F5p3rmiWPIYgZ" {
		t.Errorf("asked %v", v)
	}
}

// Older players read Liked Songs from the library.
func TestLikedSongsFromTheLibrary(t *testing.T) {
	s := newSpotify(t)
	library := &Session{Token: "Bearer t", Hashes: map[string]string{"fetchLibraryTracks": "h4", "fetchPlaylist": "h1"}}
	l, err := s.client().Read(context.Background(), LikedSongs, library)
	if err != nil {
		t.Fatal(err)
	}
	first := l.Songs[0]
	if len(s.queries) != 1 || l.Title != "Liked Songs" || len(l.Songs) != 2 || first.ID != "1MTQHCpraD4S8g5PAFKzoj" ||
		first.Duration != 269973*time.Millisecond || first.Album != "Results May Vary" {
		t.Errorf("list %+v after %d queries", l, len(s.queries))
	}
}

func TestLikedSongsNeedTheSignedInPage(t *testing.T) {
	s := newSpotify(t)
	if _, err := s.client().Read(context.Background(), LikedSongs, nil); !errors.Is(err, ErrSignIn) {
		t.Errorf("without a session: %v", err)
	}
	s.refuse = true
	if _, err := s.client().Read(context.Background(), LikedSongs, session); !errors.Is(err, ErrSignIn) {
		t.Errorf("refused: %v", err)
	}
	if len(s.queries) != 1 {
		t.Errorf("%d queries", len(s.queries))
	}
}

func TestArtistsTopSongs(t *testing.T) {
	s := newSpotify(t)
	l, err := s.client().Read(context.Background(), Ref{Artist, "4tZwfgrHOc3mvqYlEYSvVi"}, session)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.queries) != 0 || l.Title != "Daft Punk" || l.Owner != "Daft Punk" || len(l.Songs) != 3 || l.Partial ||
		!slices.Equal(l.Songs[1].Artists, []string{"Daft Punk", "Julian Casablancas"}) || l.Songs[1].Album != "" {
		t.Errorf("list %+v after %d queries", l, len(s.queries))
	}
}
