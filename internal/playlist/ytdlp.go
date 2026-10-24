package playlist

import (
	"context"
	"strings"

	"github.com/Roham1299/snag/internal/meta"
	"github.com/Roham1299/snag/internal/probe"
)

// YtDLP lists whatever yt-dlp can: playlists, channels, seasons, albums. The prober caches
// lists, so one found by the page check is quick to fetch again.
type YtDLP struct {
	Prober *probe.Prober
}

// List implements Lister.
func (y YtDLP) List(ctx context.Context, req Request) (Playlist, error) {
	info, err := y.Prober.List(ctx, probe.Request{
		URL: req.URL, Headers: req.Headers, Cookies: req.Cookies, Fresh: req.Fresh, Max: req.Max,
	})
	if err != nil {
		return Playlist{}, err
	}
	if !info.IsList() {
		return Playlist{}, ErrNotAList
	}
	p := Playlist{
		Title:   strings.TrimSpace(info.Title),
		Site:    info.Site,
		Page:    req.URL,
		Current: -1,
		Headers: req.Headers,
		Cookies: req.Cookies,
	}
	// Channel ids (UC…, 24 chars) list newest first; channels and playlists have RSS feeds.
	if info.Site == "YoutubeTab" {
		switch id := info.ID; {
		case strings.HasPrefix(id, "UC") && len(id) == 24:
			p.NewestFirst = true
			p.Feed = "https://www.youtube.com/feeds/videos.xml?channel_id=" + id
		case strings.HasPrefix(id, "PL") || strings.HasPrefix(id, "UU") || strings.HasPrefix(id, "OL"):
			p.Feed = "https://www.youtube.com/feeds/videos.xml?playlist_id=" + id
		}
	}
	playing := info.EntryFor(req.URL)
	for n, e := range info.Entries {
		if e.URL == "" {
			continue
		}
		if n == playing {
			p.Current = len(p.Entries)
		}
		p.Entries = append(p.Entries, Entry{
			URL: e.URL, Title: strings.TrimSpace(e.Title), Duration: e.Duration,
			Meta:    meta.Label{Defaults: meta.Tags{Album: p.Title, Track: len(p.Entries) + 1}},
			Waiting: e.LiveStatus == "is_upcoming" || e.LiveStatus == "is_live",
		})
	}
	if len(p.Entries) < 2 {
		return Playlist{}, ErrNotAList
	}
	return p, nil
}
