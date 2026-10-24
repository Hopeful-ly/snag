package tmdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func serve(t *testing.T, check func(*http.Request)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		check(r)
		switch r.URL.Path {
		case "/3/tv/95350":
			_, _ = w.Write([]byte(`{"id": 95350, "name": "Lanterns", "seasons": [{"season_number": 1, "name": "Season 1", "episode_count": 8}]}`))
		case "/3/tv/95350/season/1":
			_, _ = w.Write([]byte(`{"season_number": 1, "episodes": [
				{"episode_number": 1, "name": "Pilot", "runtime": 57, "air_date": "2026-08-16"},
				{"episode_number": 2, "name": "Trust Fall", "runtime": 52, "air_date": "2099-01-01"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestShowAndSeason(t *testing.T) {
	srv := serve(t, func(r *http.Request) {
		if r.URL.Query().Has("api_key") || r.Header.Get("Authorization") != "" {
			t.Errorf("a copy was sent a key: %s", r.URL)
		}
	})
	c := Client{Base: srv.URL + "/"}
	show, err := c.Show(context.Background(), 95350)
	if err != nil || show.Name != "Lanterns" || len(show.Seasons) != 1 || show.Seasons[0].Count != 8 {
		t.Fatalf("Show = %+v, %v", show, err)
	}
	season, err := c.Season(context.Background(), 95350, 1)
	if err != nil || len(season.Episodes) != 2 {
		t.Fatalf("Season = %+v, %v", season, err)
	}
	pilot := season.Episodes[0]
	if pilot.Season != 1 || pilot.Number != 1 || pilot.Runtime != 57 {
		t.Errorf("pilot %+v", pilot)
	}
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	if !pilot.Aired(now) || season.Episodes[1].Aired(now) || (Episode{}).Aired(now) {
		t.Error("aired wrong")
	}
	if _, err := c.Show(context.Background(), 1); err == nil {
		t.Error("a missing show gave no error")
	}
}

func TestKeys(t *testing.T) {
	var got *http.Request
	srv := serve(t, func(r *http.Request) { got = r })
	if _, err := (Client{Base: srv.URL, Key: "abc123"}).Show(context.Background(), 95350); err != nil {
		t.Fatal(err)
	}
	if got.URL.Query().Get("api_key") != "abc123" {
		t.Errorf("a v3 key went as %s", got.URL)
	}
	if _, err := (Client{Base: srv.URL, Key: "eyJtoken"}).Show(context.Background(), 95350); err != nil {
		t.Fatal(err)
	}
	if got.Header.Get("Authorization") != "Bearer eyJtoken" || got.URL.Query().Has("api_key") {
		t.Errorf("a v4 token went as %s, %q", got.URL, got.Header.Get("Authorization"))
	}
	// A key in Base's query must be sent with every request.
	if _, err := (Client{Base: srv.URL + "/?api_key=page"}).Season(context.Background(), 95350, 1); err != nil {
		t.Fatal(err)
	}
	if got.URL.Path != "/3/tv/95350/season/1" || got.URL.Query().Get("api_key") != "page" {
		t.Errorf("asked %s", got.URL)
	}
	if _, err := (Client{Base: "file:///etc"}).Show(context.Background(), 1); err == nil {
		t.Error("a file address was read")
	}
}
