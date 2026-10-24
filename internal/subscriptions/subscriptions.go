// Package subscriptions periodically re-reads followed lists (channels, playlists, podcasts)
// and downloads entries not seen before.
package subscriptions

import (
	"cmp"
	"hash/fnv"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Roham1299/snag/internal/naming"
	"github.com/Roham1299/snag/internal/playlist"
)

// Bounds for Subscription.EveryHours, and how many seen keys are kept.
const (
	MinEveryHours     = 1
	MaxEveryHours     = 7 * 24
	DefaultEveryHours = 6
	maxSeen           = 5000
)

// Subscription is a followed list, how its new entries download, and what has been seen.
type Subscription struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Source string `json:"source"`
	Site   string `json:"site,omitempty"`
	// Feed is a cheap way to read the newest entries; "" for none.
	Feed string `json:"feed,omitempty"`
	// NewestFirst means reading the start of the list is enough.
	NewestFirst bool `json:"newestFirst,omitempty"`
	Podcast     bool `json:"podcast,omitempty"`

	// As in playlist.Choice.
	Quality   string          `json:"quality"`
	Directory string          `json:"directory"`
	Queue     string          `json:"queue"`
	Naming    naming.Template `json:"naming,omitempty"`

	EveryHours int     `json:"everyHours"`
	Paused     bool    `json:"paused,omitempty"`
	Filters    Filters `json:"filters"`

	Created time.Time `json:"created"`
	Checked time.Time `json:"checked,omitzero"`
	Next    time.Time `json:"next"`
	Error   string    `json:"error,omitempty"` // of the last check
	Fails   int       `json:"fails,omitempty"` // consecutive
	// Added counts all downloads added; Last those added by the latest check.
	Added int `json:"added,omitempty"`
	Last  int `json:"last,omitempty"`

	// Seen holds entry keys (see EntryKey), oldest first, that never download again.
	Seen []string `json:"seen"`
}

// Filters exclude some of a list's new entries.
type Filters struct {
	// Include and Exclude are comma-separated title words or phrases; "" includes all.
	Include    string `json:"include,omitempty"`
	Exclude    string `json:"exclude,omitempty"`
	NoShorts   bool   `json:"noShorts,omitempty"`
	MinMinutes int    `json:"minMinutes,omitempty"` // applies only when the duration is known
}

// Lets reports whether the filters let e through.
func (f Filters) Lets(e playlist.Entry) bool {
	title := strings.ToLower(e.Title)
	if words := phrases(f.Include); len(words) > 0 && !slices.ContainsFunc(words, func(w string) bool {
		return strings.Contains(title, w)
	}) {
		return false
	}
	if slices.ContainsFunc(phrases(f.Exclude), func(w string) bool { return strings.Contains(title, w) }) {
		return false
	}
	if f.NoShorts && (strings.Contains(e.URL, "/shorts/") || strings.Contains(strings.ToLower(e.Title), "#shorts")) {
		return false
	}
	if f.MinMinutes > 0 && e.Duration > 0 && e.Duration < float64(f.MinMinutes*60) {
		return false
	}
	return true
}

func phrases(list string) []string {
	var out []string
	for p := range strings.SplitSeq(strings.ToLower(list), ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// youTubeVideo extracts the video id from watch?v=, youtu.be/, shorts/, live/ and embed/ URLs.
var youTubeVideo = regexp.MustCompile(`(?i)^https?://(?:(?:www|m|music)\.)?(?:youtube\.com/(?:watch\?(?:.*&)?v=|shorts/|live/|embed/)|youtu\.be/)([\w-]{11})`)

// Key returns a stable key for an entry URL: a YouTube video's id, since its feed and yt-dlp
// write the URL differently, or else the URL stripped of tracking parameters.
func Key(address string) string {
	if m := youTubeVideo.FindStringSubmatch(address); m != nil {
		return "yt:" + m[1]
	}
	u, err := url.Parse(address)
	if err != nil {
		return address
	}
	u.Fragment, u.RawFragment = "", ""
	q := u.Query()
	for name := range q {
		if strings.HasPrefix(name, "utm_") {
			q.Del(name)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// EntryKey returns e.ID if set, else Key of its URL.
func EntryKey(e playlist.Entry) string {
	if e.ID != "" {
		return "id:" + e.ID
	}
	return Key(e.URL)
}

func (s *Subscription) see(keys []string) {
	known := make(map[string]bool, len(s.Seen))
	for _, k := range s.Seen {
		known[k] = true
	}
	for _, k := range keys {
		if !known[k] {
			known[k] = true
			s.Seen = append(s.Seen, k)
		}
	}
	if len(s.Seen) > maxSeen {
		s.Seen = slices.Clone(s.Seen[len(s.Seen)-maxSeen:])
	}
}

// New subscribes to p, marking its current entries seen so only later ones download.
func New(p playlist.Playlist, c playlist.Choice, everyHours int, now time.Time) Subscription {
	s := Subscription{
		Title: cmp.Or(p.Title, "Playlist"), Source: p.Page, Site: p.Site, Feed: p.Feed,
		NewestFirst: p.NewestFirst, Podcast: p.Podcast,
		Quality: c.Quality, Directory: c.Directory, Queue: c.Queue, Naming: c.Naming,
		EveryHours: clampHours(everyHours), Created: now, Checked: now,
	}
	keys := make([]string, 0, len(p.Entries))
	for _, e := range p.Entries {
		if !e.Waiting { // so unfinished premieres still download later
			keys = append(keys, EntryKey(e))
		}
	}
	s.see(keys)
	s.Next = s.after(now)
	return s
}

func clampHours(h int) int {
	if h <= 0 {
		return DefaultEveryHours
	}
	return min(max(h, MinEveryHours), MaxEveryHours)
}

// after schedules the next check EveryHours on, with a fixed per-subscription jitter of ±10%
// so subscriptions don't all check at once.
func (s Subscription) after(now time.Time) time.Time {
	every := time.Duration(clampHours(s.EveryHours)) * time.Hour
	h := fnv.New32a()
	_, _ = h.Write([]byte(s.ID + s.Source))
	spread := float64(h.Sum32()%2001)/10000 - 0.1 // -0.1 … +0.1
	return now.Add(every + time.Duration(spread*float64(every)))
}

// retryAfter backs off exponentially from an hour, capped at a day and at EveryHours.
func (s Subscription) retryAfter(now time.Time, fails int) time.Time {
	wait := time.Hour << min(max(fails-1, 0), 5)
	wait = min(wait, 24*time.Hour, time.Duration(clampHours(s.EveryHours))*time.Hour)
	return now.Add(wait)
}

// Due reports whether s should be checked at now.
func (s Subscription) Due(now time.Time) bool {
	return !s.Paused && !now.Before(s.Next)
}
