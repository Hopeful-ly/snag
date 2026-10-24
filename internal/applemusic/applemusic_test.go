package applemusic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	song, ok := Parse("https://music.apple.com/us/album/bitab-live/1545371283?i=1545371999&l=en")
	if !ok || song.Kind != Album || song.ID != "1545371283" || song.Song != "1545371999" || !song.One() ||
		song.Address != "https://music.apple.com/us/album/bitab-live/1545371283?i=1545371999" {
		t.Fatalf("song in an album: %+v %v", song, ok)
	}
	pl, ok := Parse("https://geo.music.apple.com/us/playlist/todays-hits/pl.f4d106fed2bd41149aaacabb233eb5eb")
	if !ok || pl.Kind != Playlist || pl.Address != "https://music.apple.com/us/playlist/todays-hits/pl.f4d106fed2bd41149aaacabb233eb5eb" {
		t.Fatalf("playlist: %+v %v", pl, ok)
	}
	if _, ok := Parse("https://music.apple.com/us/artist/mehrad-hidden/632147450"); !ok {
		t.Fatal("artist")
	}
	if _, ok := Parse("https://music.apple.com/us/song/bitab-live/1545371999"); !ok {
		t.Fatal("song")
	}
	if _, ok := Parse("https://open.spotify.com/track/3zOYmiNOuyDL0ClxJcEJ0T"); ok {
		t.Fatal("spotify is not apple music")
	}
	if _, ok := Parse("https://music.apple.com/us/library/playlist/p.123"); ok {
		t.Fatal("a library page is not a catalog list")
	}
}

func TestRead(t *testing.T) {
	files := map[string]string{
		"/us/album/live/1545371283":                "album.html",
		"/us/playlist/todays-hits/pl.aaaaaaaaaaaa": "playlist.html",
		"/us/artist/mehrad-hidden/632147450":       "artist.html",
		"/us/song/bitab-live/1545371999":           "song.html",
	}
	lookup := `{"resultCount":2,"results":[
		{"wrapperType":"collection","collectionName":"Live"},
		{"wrapperType":"track","trackId":1545371448,"trackName":"Barf (Live)","artistName":"Mehrad Hidden","collectionName":"Mehrad Hidden Online Performance in Tehran (Live)","collectionArtistName":"Mehrad Hidden","trackNumber":1,"discNumber":1,"releaseDate":"2020-12-16T12:00:00Z","trackTimeMillis":228693,"artworkUrl100":"https://is1-ssl.mzstatic.com/image/thumb/cover/100x100bb.jpg"},
		{"wrapperType":"track","trackId":1545371999,"trackName":"Bitab (Live)","artistName":"Mehrad Hidden","collectionName":"Mehrad Hidden Online Performance in Tehran (Live)","collectionArtistName":"Mehrad Hidden","trackNumber":4,"discNumber":2,"releaseDate":"2020-12-16T12:00:00Z","trackTimeMillis":270336,"artworkUrl100":"https://is1-ssl.mzstatic.com/image/thumb/cover/100x100bb.jpg"}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/lookup" {
			_, _ = w.Write([]byte(lookup))
			return
		}
		file := files[r.URL.Path]
		if file == "" {
			http.NotFound(w, r)
			return
		}
		b, err := os.ReadFile("testdata/" + file)
		if err != nil {
			t.Error(err)
		}
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)
	c := Client{HTTP: srv.Client(), Page: srv.URL, Lookup: srv.URL + "/lookup"}

	album, ok := Parse("https://music.apple.com/us/album/live/1545371283")
	if !ok {
		t.Fatal("album address")
	}
	list, err := c.Read(context.Background(), album)
	if err != nil {
		t.Fatal(err)
	}
	if list.Title != "Mehrad Hidden Online Performance in Tehran (Live)" || list.Owner != "Mehrad Hidden" ||
		len(list.Songs) != 2 || list.Total != 0 {
		t.Fatalf("album %+v", list)
	}
	bitab := list.Songs[1]
	if bitab.Title != "Bitab (Live)" || bitab.Artists[0] != "Mehrad Hidden" || bitab.Number != 4 || bitab.Disc != 2 ||
		bitab.Date != "2020-12-16" || bitab.Album != list.Title || bitab.Duration != 270336*time.Millisecond ||
		bitab.Cover != "https://is1-ssl.mzstatic.com/image/thumb/cover.jpg/1400x1400bb.jpg" ||
		bitab.URL != "https://music.apple.com/us/album/bitab-live/1545371283?i=1545371999" {
		t.Fatalf("bitab %+v", bitab)
	}

	one, _ := Parse("https://music.apple.com/us/album/live/1545371283?i=1545371999")
	only, err := c.Read(context.Background(), one)
	if err != nil {
		t.Fatal(err)
	}
	if only.Kind != Track || only.Title != "Bitab (Live)" || len(only.Songs) != 1 || only.Songs[0].Disc != 2 ||
		only.Songs[0].Date != "2020-12-16" {
		t.Fatalf("one song %+v", only)
	}

	pl, _ := Parse("https://music.apple.com/us/playlist/todays-hits/pl.aaaaaaaaaaaa")
	got, err := c.Read(context.Background(), pl)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Today's Hits" || got.Total != 4 || len(got.Songs) != 1 ||
		got.Songs[0].Artists[0] != "Olivia Rodrigo" || got.Songs[0].Album != "expectations" ||
		got.Songs[0].Cover != "https://is1-ssl.mzstatic.com/image/thumb/olivia.jpg/1400x1400bb.jpg" {
		t.Fatalf("playlist %+v", got)
	}

	artist, _ := Parse("https://music.apple.com/us/artist/mehrad-hidden/632147450")
	top, err := c.Read(context.Background(), artist)
	if err != nil {
		t.Fatal(err)
	}
	if top.Title != "Mehrad Hidden" || len(top.Songs) != 1 || top.Songs[0].Album != "Toonel, Vol. 1" ||
		top.Songs[0].Date != "2017" || top.Songs[0].Artists[0] != "Mehrad Hidden" {
		t.Fatalf("artist %+v song %+v", top, top.Songs[0])
	}

	page, _ := Parse("https://music.apple.com/us/song/bitab-live/1545371999")
	song, err := c.Read(context.Background(), page)
	if err != nil {
		t.Fatal(err)
	}
	s := song.Songs[0]
	if s.Title != "Bitab (Live)" || s.Date != "2020-12-16" || s.Duration != 270336*time.Millisecond ||
		s.Number != 4 || s.Disc != 2 || s.Cover == "" || s.Album == "" {
		t.Fatalf("song page %+v", s)
	}
}

func TestSongFromLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"resultCount":1,"results":[{"wrapperType":"track","trackId":1545371999,"trackName":"Bitab (Live)","artistName":"Mehrad Hidden","collectionName":"Live - EP","collectionArtistName":"Mehrad Hidden","trackNumber":4,"discNumber":1,"releaseDate":"2020-12-16T12:00:00Z","trackTimeMillis":270336,"artworkUrl100":"https://example.test/100x100bb.jpg"}]}`))
	}))
	t.Cleanup(srv.Close)
	c := Client{HTTP: srv.Client(), Lookup: srv.URL}
	ref, ok := Parse("https://music.apple.com/us/song/bitab-live/1545371999")
	if !ok {
		t.Fatal("parse")
	}
	song, err := c.Song(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if song.Title != "Bitab (Live)" || song.Artists[0] != "Mehrad Hidden" || song.Album != "Live" ||
		song.Number != 4 || song.Disc != 1 || song.Date != "2020-12-16" || song.Duration != 270336*time.Millisecond ||
		song.Cover != "https://example.test/1400x1400bb.jpg" {
		t.Fatalf("%+v", song)
	}
}
