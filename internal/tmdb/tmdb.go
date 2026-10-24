// Package tmdb reads TV seasons and episodes from The Movie Database v3 API or a site's mirror
// of it.
package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Official is TMDB's own API, which needs a key.
const Official = "https://api.themoviedb.org"

// Client reads one TMDB API.
type Client struct {
	// Base is the API URL without "/3". Its query, such as a mirror's "?api_key=…", is sent with
	// every request.
	Base string
	// Key is a v3 API key or a v4 read access token; "" for a mirror that needs none.
	Key  string
	HTTP *http.Client // nil for a default with a short timeout
}

// Show is a TV show and its seasons.
type Show struct {
	ID      int      `json:"id"`
	Name    string   `json:"name"`
	Seasons []Season `json:"seasons"`
}

// Season is one season of a show; Episodes is only filled by Client.Season.
type Season struct {
	Number   int       `json:"season_number"`
	Name     string    `json:"name"`
	Count    int       `json:"episode_count"`
	Episodes []Episode `json:"episodes"`
}

// Episode is one episode of a season.
type Episode struct {
	Season  int    `json:"season_number"`
	Number  int    `json:"episode_number"`
	Name    string `json:"name"`
	Runtime int    `json:"runtime"`  // minutes; 0 if unknown
	AirDate string `json:"air_date"` // YYYY-MM-DD; "" if unknown
}

// Aired reports whether the episode has a known air date no later than now.
func (e Episode) Aired(now time.Time) bool {
	day, err := time.Parse(time.DateOnly, e.AirDate)
	return err == nil && !day.After(now)
}

// Show returns the show with TMDB id.
func (c Client) Show(ctx context.Context, id int) (Show, error) {
	var s Show
	return s, c.get(ctx, fmt.Sprintf("/3/tv/%d", id), &s)
}

// Season returns a show's season, with its episodes.
func (c Client) Season(ctx context.Context, id, season int) (Season, error) {
	var s Season
	err := c.get(ctx, fmt.Sprintf("/3/tv/%d/season/%d", id, season), &s)
	for i := range s.Episodes {
		if s.Episodes[i].Season == 0 {
			s.Episodes[i].Season = season
		}
	}
	return s, err
}

func (c Client) get(ctx context.Context, path string, into any) error {
	u, err := url.Parse(c.Base)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return errors.New("not a TMDB address: " + c.Base)
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	u.RawPath, u.Fragment = "", ""
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	switch {
	case c.Key == "":
	case strings.HasPrefix(c.Key, "eyJ"): // a v4 token (JWT), which v3 accepts as a bearer
		req.Header.Set("Authorization", "Bearer "+c.Key)
	default:
		q := u.Query()
		q.Set("api_key", c.Key)
		req.URL.RawQuery = q.Encode()
	}
	req.Header.Set("Accept", "application/json")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("TMDB said %s", resp.Status)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(into)
}
