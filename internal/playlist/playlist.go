// Package playlist finds the list a page offers (playlist, channel, season, album, feed) and
// plans downloads of its entries.
package playlist

import (
	"context"
	"errors"
	"slices"

	"github.com/Roham1299/snag/internal/cookies"
	"github.com/Roham1299/snag/internal/meta"
	"github.com/Roham1299/snag/internal/spotify"
)

// ErrNotAList means the page is not a list of more than one entry, nor in one.
var ErrNotAList = errors.New("not a playlist")

// Via is how a list's entries' streams are found.
type Via string

const (
	// ViaBrowser plays each entry's page in the browser to find its stream, for sites whose
	// links expire.
	ViaBrowser Via = "browser"
	// ViaSearch finds each song on YouTube Music by name, for Spotify and Apple Music lists
	// whose audio can't be downloaded.
	ViaSearch Via = "search"
)

// Playlist is a list of videos or tracks, as a page offers it.
type Playlist struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Site    string  `json:"site,omitempty"`
	Page    string  `json:"page"`
	Entries []Entry `json:"entries"`
	// Current is the index of the entry the page plays, or -1.
	Current int `json:"current"`
	// Via is "" when entry URLs are directly downloadable.
	Via  Via    `json:"via,omitempty"`
	Note string `json:"note,omitempty"`
	// NewestFirst means new entries appear at the start, as on a channel.
	NewestFirst bool `json:"newestFirst,omitempty"`
	// Feed is a cheap address for checking for new entries, such as a YouTube RSS feed.
	Feed    string `json:"feed,omitempty"`
	Podcast bool   `json:"podcast,omitempty"`
	Follow  bool   `json:"follow,omitempty"`

	Headers map[string]string `json:"-"`
	Cookies []cookies.Cookie  `json:"-"`
}

// Entry is one video or track of a list. Title and Duration may be unknown until download.
type Entry struct {
	URL      string  `json:"url"`
	Title    string  `json:"title,omitempty"`
	Duration float64 `json:"duration,omitempty"` // seconds
	// Group is the part of the list it is in, such as "Season 2".
	Group string `json:"group,omitempty"`
	// Tags is left empty by Listers and filled by Named.
	Tags meta.Tags  `json:"tags,omitempty"`
	Meta meta.Label `json:"-"`
	// Waiting marks an unfinished premiere or live stream.
	Waiting bool `json:"waiting,omitempty"`
	// ID is a stable identifier where the URL isn't (a podcast guid, whose enclosure URL carries
	// changing tracking parameters); "" to identify it by URL.
	ID string `json:"-"`
}

// Named returns p with each entry's Tags filled for name templates.
func (p Playlist) Named() Playlist {
	p.Entries = slices.Clone(p.Entries)
	for i, e := range p.Entries {
		tags := e.Meta.Known(e.Title)
		tags.Page, tags.Cover = "", ""
		p.Entries[i].Tags = tags
	}
	return p
}

// Request asks for the list a page is in, or is.
type Request struct {
	URL     string
	Title   string
	Headers map[string]string
	Cookies []cookies.Cookie
	// TMDB is the base URL of the TMDB API mirror the page used, without "/3".
	TMDB    string
	Spotify *spotify.Session
	// Feeds are the page's <link rel="alternate"> feeds.
	Feeds []string
	// FeedsOnly skips every lister but Feed, as the page itself was already checked.
	FeedsOnly bool
	Follow    bool
	// Fresh bypasses any cached read. Max limits entries read from the start; 0 for all.
	Fresh bool
	Max   int
}

// Lister finds a page's list. It returns ErrNotAList for a page it has no list for.
type Lister interface {
	List(ctx context.Context, req Request) (Playlist, error)
}

// Listers tries each lister in turn; the first that doesn't return ErrNotAList answers.
type Listers []Lister

// List implements Lister.
func (ls Listers) List(ctx context.Context, req Request) (Playlist, error) {
	for _, l := range ls {
		if _, feed := l.(Feed); req.FeedsOnly && !feed {
			continue
		}
		p, err := l.List(ctx, req)
		if !errors.Is(err, ErrNotAList) {
			return p, err
		}
	}
	return Playlist{}, ErrNotAList
}
