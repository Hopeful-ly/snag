package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Roham1299/snag/internal/applemusic"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/pagedl"
	"github.com/Roham1299/snag/internal/spotify"
	"github.com/Roham1299/snag/internal/ytmusic"
)

const ashe = "https://open.spotify.com/track/3zOYmiNOuyDL0ClxJcEJ0T"

func spotifySong(t *testing.T, search ytmusic.Searcher) *Service {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embed/track/3zOYmiNOuyDL0ClxJcEJ0T" {
			http.NotFound(w, r)
			return
		}
		b, _ := os.ReadFile("../spotify/testdata/embed-track.html")
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return &Service{spotify: spotify.Client{HTTP: srv.Client(), Open: srv.URL}, songs: ytmusic.Finder{Search: search}}
}

func TestResolveFindsSpotifySongs(t *testing.T) {
	var searched []string
	s := spotifySong(t, func(_ context.Context, query string, _ int) ([]ytmusic.Result, error) {
		searched = append(searched, query)
		return []ytmusic.Result{{Title: "Stop The Wedding!", Track: "Stop The Wedding!", Artists: []string{"Ashe"},
			Duration: 199, WebpageURL: "https://music.youtube.com/watch?v=WNxAvQNsJuM"}}, nil
	})
	src, err := s.Resolve(context.Background(), pagedl.Request{Page: ashe})
	if err != nil {
		t.Fatal(err)
	}
	if src.URL != "https://music.youtube.com/watch?v=WNxAvQNsJuM" || src.Tags.Artist != "Ashe" ||
		src.Tags.Date != "2026-08-07" || !strings.Contains(src.Tags.Cover, "b273") || len(searched) != 1 {
		t.Errorf("resolved %+v after %q", src, searched)
	}
}

func TestNoMatchAsksUnlessTheQueueNamesAPlace(t *testing.T) {
	none := func(context.Context, string, int) ([]ytmusic.Result, error) { return nil, nil }
	_, err := spotifySong(t, none).Resolve(context.Background(), pagedl.Request{Page: ashe})
	var ask *engine.NeedsPlace
	if !errors.As(err, &ask) || !strings.Contains(err.Error(), "no match") {
		t.Fatalf("no match: %v", err)
	}

	var asked []string
	s := spotifySong(t, func(_ context.Context, query string, _ int) ([]ytmusic.Result, error) {
		asked = append(asked, query)
		if strings.HasPrefix(query, "scsearch") {
			return []ytmusic.Result{{Title: "Stop The Wedding!", Track: "Stop The Wedding!", Artists: []string{"Ashe"},
				Duration: 199, WebpageURL: "https://soundcloud.com/ashe/stop"}}, nil
		}
		return nil, nil
	})
	src, err := s.Resolve(context.Background(), pagedl.Request{Page: ashe, Fallback: ytmusic.PlaceSoundCloud})
	if err != nil || src.URL != "https://soundcloud.com/ashe/stop" || len(asked) != 3 ||
		!strings.HasPrefix(asked[2], "scsearch8:") {
		t.Errorf("fallback %+v, %v after %q", src, err, asked)
	}
}

func TestResolveFindsAppleMusicSongs(t *testing.T) {
	const page = "https://music.apple.com/us/song/bitab-live/1545371999"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"resultCount":1,"results":[{"wrapperType":"track","trackId":1545371999,"trackName":"Bitab (Live)","artistName":"Mehrad Hidden","collectionName":"Mehrad Hidden Online Performance in Tehran (Live)","collectionArtistName":"Mehrad Hidden","trackNumber":4,"discNumber":1,"releaseDate":"2020-12-16T12:00:00Z","trackTimeMillis":270336,"artworkUrl100":"https://is1-ssl.mzstatic.com/image/thumb/cover/100x100bb.jpg"}]}`))
	}))
	t.Cleanup(srv.Close)
	var searched []string
	s := &Service{
		apple: applemusic.Client{HTTP: srv.Client(), Lookup: srv.URL},
		songs: ytmusic.Finder{Search: func(_ context.Context, query string, _ int) ([]ytmusic.Result, error) {
			searched = append(searched, query)
			return []ytmusic.Result{{Title: "Bitab (Live)", Track: "Bitab (Live)", Artists: []string{"Mehrad Hidden"},
				Duration: 270, WebpageURL: "https://music.youtube.com/watch?v=Kxeu3ZSE6DQ"}}, nil
		}},
	}
	src, err := s.Resolve(context.Background(), pagedl.Request{Page: page})
	if err != nil {
		t.Fatal(err)
	}
	if src.URL != "https://music.youtube.com/watch?v=Kxeu3ZSE6DQ" || src.Tags.Title != "Bitab (Live)" ||
		src.Tags.Artist != "Mehrad Hidden" || src.Tags.Track != 4 || src.Tags.Disc != 1 ||
		src.Tags.Date != "2020-12-16" || !strings.Contains(src.Tags.Cover, "1400x1400") || len(searched) == 0 {
		t.Errorf("resolved %+v after %q", src, searched)
	}
}
