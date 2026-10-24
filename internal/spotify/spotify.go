// Package spotify reads song lists from Spotify, for finding each song elsewhere.
// Spotify's sound is DRM-protected and is never fetched.
//
// Embed pages need no account but stop at 100 songs and omit albums; the web player's API,
// asked with a Session captured by the extension, has everything.
package spotify

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Kind is the kind of page a Spotify address points at.
type Kind string

// Kinds of Spotify page.
const (
	Playlist Kind = "playlist"
	Album    Kind = "album"
	Track    Kind = "track"
	Artist   Kind = "artist"     // their top songs
	Liked    Kind = "collection" // Liked Songs, at /collection/tracks
)

// Noun names the kind for use in a sentence.
func (k Kind) Noun() string {
	switch k {
	case Track:
		return "the song"
	case Artist:
		return "the artist's top songs"
	case Liked:
		return "your Liked Songs"
	}
	return "the " + string(k)
}

// Ref names a Spotify page.
type Ref struct {
	Kind Kind
	ID   string // "tracks" for Liked Songs
}

// LikedSongs is the Ref of the user's Liked Songs.
var LikedSongs = Ref{Liked, "tracks"}

// URL is the address of its page.
func (r Ref) URL() string {
	return "https://open.spotify.com/" + string(r.Kind) + "/" + r.ID
}

func (r Ref) uri() string {
	return "spotify:" + string(r.Kind) + ":" + r.ID
}

var (
	pagePath = regexp.MustCompile(`^/(?:intl-[a-z]{2}(?:-[A-Za-z]{2})?/)?(?:embed/)?(?:(playlist|album|track|artist)/([A-Za-z0-9]{22})|(collection/tracks))/?$`)
	uriForm  = regexp.MustCompile(`^spotify:(?:(playlist|album|track|artist):([A-Za-z0-9]{22})|(collection:tracks|user:[^:]+:collection))$`)
)

// Parse reads a Spotify page address or spotify: URI and reports whether it is one.
func Parse(address string) (Ref, bool) {
	address = strings.TrimSpace(address)
	m := uriForm.FindStringSubmatch(address)
	if m == nil {
		u, err := url.Parse(address)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() != "open.spotify.com" {
			return Ref{}, false
		}
		m = pagePath.FindStringSubmatch(u.Path)
	}
	switch {
	case m == nil:
		return Ref{}, false
	case m[3] != "":
		return LikedSongs, true
	}
	return Ref{Kind(m[1]), m[2]}, true
}

// Song is one of a list's songs.
type Song struct {
	ID           string
	Title        string
	Artists      []string
	Album        string // "" when read from an embed
	AlbumArtists []string
	Cover        string // the largest album cover
	Date         string // YYYY-MM-DD, or YYYY alone
	Disc, Number int    // 0 when unknown
	Duration     time.Duration
}

// List is a playlist, album or single song, with its songs in order.
type List struct {
	Ref
	Title string
	Owner string // a playlist's owner, or an album's artists
	Cover string
	Songs []Song
	// Partial means songs are missing: embeds stop at 100.
	Partial bool
}

// Session is the credentials an open web player page sent Spotify's API, replayed to read it.
type Session struct {
	Token       string            `json:"token"` // "Bearer …"
	ClientToken string            `json:"client_token"`
	AppVersion  string            `json:"app_version"`
	UserAgent   string            `json:"user_agent"`
	Hashes      map[string]string `json:"hashes"` // persisted query name to hash
}

// Usable reports whether s has enough to make a request.
func (s *Session) Usable() bool {
	return s != nil && s.Token != "" && len(s.Hashes) > 0
}

// Spotify base URLs.
const (
	OpenBase = "https://open.spotify.com"
	APIBase  = "https://api-partner.spotify.com"
)

// Client reads Spotify.
type Client struct {
	HTTP *http.Client // nil for a 20 s timeout
	Open string       // "" for OpenBase
	API  string       // "" for APIBase
}

// ErrNotFound means Spotify has no such playlist, album or song, or won't show it.
var ErrNotFound = errors.New("there's nothing on Spotify at that address, or it is private")

// ErrSignIn means the list (Liked Songs) needs a signed-in session.
var ErrSignIn = errors.New("your Liked Songs are read through Spotify's page in your browser: " +
	"open them there, signed in, and use the Snag button on the page")

// errSession means the session was refused (expired, or a query hash changed); fall back to embeds.
var errSession = errors.New("the browser's Spotify session was refused")

// Read reads ref's list through the session when usable, else from the embeds.
func (c Client) Read(ctx context.Context, ref Ref, s *Session) (List, error) {
	switch ref.Kind {
	case Liked:
		if !s.Usable() {
			return List{}, ErrSignIn
		}
		list, err := c.Web(ctx, ref, *s)
		if errors.Is(err, errSession) {
			return List{}, fmt.Errorf("%w (%w)", ErrSignIn, err)
		}
		return list, err
	case Playlist, Album:
		if !s.Usable() {
			break
		}
		list, err := c.Web(ctx, ref, *s)
		if err == nil {
			return c.named(ctx, list), nil
		}
		if ctx.Err() != nil {
			return List{}, err
		}
	}
	return c.Embed(ctx, ref)
}

// named fills the title, owner and cover from the embed when the API query left them out.
func (c Client) named(ctx context.Context, list List) List {
	if list.Title != "" {
		return list
	}
	if e, err := c.Embed(ctx, list.Ref); err == nil {
		list.Title, list.Owner, list.Cover = e.Title, e.Owner, e.Cover
	}
	return list
}

func (c Client) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 20 * time.Second}
}

func (c Client) do(req *http.Request) (*http.Response, error) {
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("asking Spotify: %w", err)
	}
	if resp.StatusCode == http.StatusOK {
		return resp, nil
	}
	resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusBadRequest:
		return nil, fmt.Errorf("%w (%s)", errSession, resp.Status)
	}
	return nil, fmt.Errorf("asking Spotify: %s", resp.Status)
}

func largest(sources []picture) string {
	best, size := "", -1
	for _, p := range sources {
		if p.URL != "" && p.Width > size {
			best, size = p.URL, p.Width
		}
	}
	return best
}

type picture struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func idOf(uri string) string {
	return uri[strings.LastIndexByte(uri, ':')+1:]
}

// day is YYYY-MM-DD, or YYYY when precision is YEAR.
func day(iso, precision string) string {
	if len(iso) < 10 {
		return ""
	}
	if precision == "YEAR" {
		return iso[:4]
	}
	return iso[:10]
}
