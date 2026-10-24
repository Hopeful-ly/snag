package ytdlp

import (
	"context"
	"net/url"
	"regexp"
	"strings"
)

// List runs yt-dlp to flat-list the playlist a page is in, or is. Entries carry little beyond
// URL and id; a page outside any playlist is described in full, as by Describe.
func List(ctx context.Context, ytdlp string, o InfoOptions) (Info, error) {
	o.Flat = true
	o.URL = listAddress(o.URL)
	return describe(ctx, ytdlp, o)
}

// listAddress rewrites YouTube URLs to the page yt-dlp lists best. watch?v=…&list=PL… goes to
// /playlist (the watch page reports a nameless "show"); mixes (list=RD…) only exist beside a
// video, so they stay. A channel's home goes to its Videos tab, since yt-dlp lists the home as
// its tabs, which aren't downloadable.
func listAddress(page string) string {
	u, err := url.Parse(page)
	if err != nil || !isYouTube(u.Hostname()) || u.Path == "/playlist" {
		return page
	}
	if home := channelHome.FindStringSubmatch(u.Path); home != nil {
		videos := *u
		videos.Path, videos.RawQuery, videos.Fragment = home[1]+"/videos", "", ""
		return videos.String()
	}
	list := u.Query().Get("list")
	if list == "" || strings.HasPrefix(list, "RD") {
		return page
	}
	host := u.Host
	if host == "youtu.be" {
		host = "www.youtube.com"
	}
	return (&url.URL{Scheme: "https", Host: host, Path: "/playlist", RawQuery: url.Values{"list": {list}}.Encode()}).String()
}

// channelHome matches a YouTube channel's home (or /featured) tab; group 1 is the channel path.
var channelHome = regexp.MustCompile(`^(/@[^/]+|/(?:channel|c|user)/[^/]+)/?(?:featured/?)?$`)

func isYouTube(host string) bool {
	return host == "youtu.be" || host == "youtube.com" || strings.HasSuffix(host, ".youtube.com")
}

// IsList reports whether info is a playlist of more than one entry.
func (i Info) IsList() bool {
	return i.Type == "playlist" && len(i.Entries) > 1
}

// EntryFor returns the index of the entry page points at, or -1. It matches the entry's URL, or
// its id as a query value or path segment (watch?v=ID&list=…).
func (i Info) EntryFor(page string) int {
	u, err := url.Parse(page)
	if err != nil {
		return -1
	}
	for n, e := range i.Entries {
		if e.URL != "" && sameAddress(e.URL, u) {
			return n
		}
	}
	for n, e := range i.Entries {
		if e.ID != "" && mentions(u, e.ID) {
			return n
		}
	}
	return -1
}

func sameAddress(address string, page *url.URL) bool {
	u, err := url.Parse(address)
	return err == nil && strings.EqualFold(u.Host, page.Host) &&
		strings.TrimSuffix(u.Path, "/") == strings.TrimSuffix(page.Path, "/") &&
		(u.RawQuery == "" || u.RawQuery == page.RawQuery)
}

func mentions(page *url.URL, id string) bool {
	for _, values := range page.Query() {
		for _, v := range values {
			if v == id {
				return true
			}
		}
	}
	for _, part := range strings.Split(page.Path, "/") {
		if part == id {
			return true
		}
	}
	return false
}
