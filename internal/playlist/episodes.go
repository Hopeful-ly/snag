package playlist

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Roham1299/snag/internal/meta"
	"github.com/Roham1299/snag/internal/tmdb"
	"github.com/Roham1299/snag/internal/ytdlp"
)

// Episodes lists a TV show's episodes on sites whose URLs use TMDB ids (/tv/95350,
// /tv/95350/1/3), via the TMDB API mirror the page loaded or else TMDB with the user's key.
// Entries are played in the browser (ViaBrowser) since such players fetch streams on demand.
type Episodes struct {
	Key  func() string    // nil or "" for no key
	TMDB string           // "" for tmdb.Official
	HTTP *http.Client     // nil for tmdb's default
	Now  func() time.Time // nil for time.Now
}

// showPage captures prefix, show id, and optional season and episode.
var showPage = regexp.MustCompile(`^(.*/(?:tv|series|show))/(\d+)(?:/(\d+)/(\d+))?/?$`)

// List implements Lister.
func (l Episodes) List(ctx context.Context, req Request) (Playlist, error) {
	page, err := url.Parse(req.URL)
	if err != nil {
		return Playlist{}, ErrNotAList
	}
	m := showPage.FindStringSubmatch(page.Path)
	if m == nil {
		return Playlist{}, ErrNotAList
	}
	id, _ := strconv.Atoi(m[2])
	season, _ := strconv.Atoi(m[3])
	episode, _ := strconv.Atoi(m[4])

	clients := l.clients(req.TMDB)
	if len(clients) == 0 {
		return Playlist{}, ErrNotAList
	}
	var show tmdb.Show
	var client tmdb.Client
	for _, c := range clients {
		if show, err = c.Show(ctx, id); err == nil {
			client = c
			break
		}
	}
	if err != nil {
		return Playlist{}, fmt.Errorf("couldn't read the show from TMDB: %w", err)
	}
	if req.TMDB == "" && !sameShow(show.Name, req.Title) {
		// A bare numeric id could be anything; require the page title to name the show.
		return Playlist{}, ErrNotAList
	}

	p := Playlist{
		Title:   strings.TrimSpace(show.Name),
		Site:    page.Hostname(),
		Page:    req.URL,
		Current: -1,
		Via:     ViaBrowser,
		Headers: req.Headers,
		Cookies: req.Cookies,
	}
	now := time.Now()
	if l.Now != nil {
		now = l.Now()
	}
	for _, s := range show.Seasons {
		if s.Number < 1 || len(p.Entries) >= ytdlp.MaxEntries {
			continue // season 0 holds specials, which sites number their own ways
		}
		full, err := client.Season(ctx, id, s.Number)
		if err != nil {
			return Playlist{}, fmt.Errorf("couldn't read %s's season %d from TMDB: %w", p.Title, s.Number, err)
		}
		for _, e := range full.Episodes {
			if !e.Aired(now) || len(p.Entries) >= ytdlp.MaxEntries {
				continue
			}
			if e.Season == season && e.Number == episode {
				p.Current = len(p.Entries)
			}
			p.Entries = append(p.Entries, l.entry(p.Title, page, m[1]+"/"+m[2], e))
		}
	}
	if len(p.Entries) < 2 {
		return Playlist{}, ErrNotAList
	}
	return p, nil
}

func (l Episodes) clients(copyBase string) []tmdb.Client {
	var cs []tmdb.Client
	if u, err := url.Parse(copyBase); err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" {
		cs = append(cs, tmdb.Client{Base: copyBase, HTTP: l.HTTP})
	}
	if l.Key != nil {
		if key := strings.TrimSpace(l.Key()); key != "" {
			cs = append(cs, tmdb.Client{Base: cmp.Or(l.TMDB, tmdb.Official), Key: key, HTTP: l.HTTP})
		}
	}
	return cs
}

// entry tags the episode with Only, since its stream carries nothing but a name.
func (l Episodes) entry(show string, page *url.URL, prefix string, e tmdb.Episode) Entry {
	u := *page
	u.Path = fmt.Sprintf("%s/%d/%d", prefix, e.Season, e.Number)
	u.RawPath, u.Fragment = "", "" // keep the query: the site needs it to start playing

	title := fmt.Sprintf("S%d E%d", e.Season, e.Number)
	name := strings.TrimSpace(e.Name)
	if name != "" {
		title += " · " + name
	}
	return Entry{
		URL:      u.String(),
		Title:    title,
		Duration: float64(e.Runtime * 60),
		Group:    fmt.Sprintf("Season %d", e.Season),
		Meta: meta.Label{Only: true, Tags: meta.Tags{
			Title:   cmp.Or(name, fmt.Sprintf("Episode %d", e.Number)),
			Show:    show,
			Season:  e.Season,
			Episode: e.Number,
			Date:    e.AirDate,
			Page:    u.String(),
		}},
	}
}

// sameShow reports whether pageTitle names the show, e.g. "Lanterns | Example" for "Lanterns".
func sameShow(name, pageTitle string) bool {
	words := func(s string) string {
		return strings.Join(strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		}), " ")
	}
	n := words(name)
	return n != "" && strings.Contains(" "+words(pageTitle)+" ", " "+n+" ")
}
