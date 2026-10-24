package playlist

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Roham1299/snag/internal/meta"
)

const (
	maxFeedSize = 64 << 20 // long-running daily shows' feeds exceed 20 MB
	feedTimeout = 30 * time.Second
)

const maxFeedEntries = 1000

// feedAddress gates fetching a page as a feed, so ordinary pages cost no extra request.
var feedAddress = regexp.MustCompile(`(?i)(\.(rss|xml|atom)$|/(rss|feed|feeds|podcast|atom)(/|$|\.)|[?&](format|type)=(rss|atom)|^feeds\.|\.libsyn\.com$|anchor\.fm$|buzzsprout\.com$|podbean\.com$|megaphone\.fm$|simplecast\.com$|transistor\.fm$|soundcloud\.com/.*/sounds\.rss$)`)

// LooksLikeFeed reports whether address may be a podcast's or a site's feed (RSS or Atom).
func LooksLikeFeed(address string) bool {
	u, err := url.Parse(address)
	if err != nil {
		return false
	}
	return feedAddress.MatchString(u.Path) || feedAddress.MatchString("?"+u.RawQuery) ||
		feedAddress.MatchString(u.Hostname())
}

// Feed lists a podcast's episodes, or any feed's videos and files, from its RSS or Atom feed.
type Feed struct {
	Client *http.Client // nil for a default client with feedTimeout
}

// maxPageFeeds caps the linked feeds read: a site's main feed comes first, comment and
// category feeds after.
const maxPageFeeds = 3

// List implements Lister for feed addresses and for pages that link to a podcast feed.
func (f Feed) List(ctx context.Context, req Request) (Playlist, error) {
	if !LooksLikeFeed(req.URL) || req.FeedsOnly {
		return f.fromPage(ctx, req)
	}
	p, err := f.Read(ctx, req.URL)
	if err != nil {
		return Playlist{}, err
	}
	p.Headers, p.Cookies = req.Headers, req.Cookies
	return p, nil
}

// fromPage returns the first linked feed that is a podcast's; article feeds have nothing to
// download, and YouTube channel feeds are handled by the YouTube lister.
func (f Feed) fromPage(ctx context.Context, req Request) (Playlist, error) {
	read := 0
	for _, feed := range req.Feeds {
		if !isWebAddress(feed) || feed == req.URL || read == maxPageFeeds {
			continue
		}
		read++
		p, err := f.Read(ctx, feed)
		if err == nil && p.Podcast {
			return p, nil
		}
		if ctx.Err() != nil {
			return Playlist{}, ctx.Err()
		}
	}
	return Playlist{}, ErrNotAList
}

// Read reads the feed at address without checking that the address looks like a feed.
func (f Feed) Read(ctx context.Context, address string) (Playlist, error) {
	data, err := f.fetch(ctx, address)
	if err != nil {
		return Playlist{}, err
	}
	return ParseFeed(data, address)
}

func isWebAddress(address string) bool {
	u, err := url.Parse(address)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func (f Feed) fetch(ctx context.Context, address string) ([]byte, error) {
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: feedTimeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Snag (podcasts)")
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml;q=0.9, */*;q=0.5")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the feed's site answered %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFeedSize {
		return nil, errors.New("the feed is too large for Snag to read")
	}
	return data, nil
}

// feedDoc decodes both RSS 2.0 (with iTunes podcast tags) and Atom.
type feedDoc struct {
	XMLName xml.Name
	// RSS
	Channel struct {
		Title  string     `xml:"title"`
		Link   string     `xml:"link"`
		Author string     `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd author"`
		Image  feedImage  `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd image"`
		Items  []feedItem `xml:"item"`
	} `xml:"channel"`
	// Atom
	Title   string      `xml:"title"`
	Author  feedPerson  `xml:"author"`
	Entries []feedEntry `xml:"entry"`
}

type feedImage struct {
	Href string `xml:"href,attr"`
}

type feedPerson struct {
	Name string `xml:"name"`
}

type feedItem struct {
	Title     string `xml:"title"`
	Link      string `xml:"link"`
	GUID      string `xml:"guid"`
	PubDate   string `xml:"pubDate"`
	Enclosure struct {
		URL  string `xml:"url,attr"`
		Type string `xml:"type,attr"`
	} `xml:"enclosure"`
	Duration string    `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd duration"`
	Episode  string    `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd episode"`
	Season   string    `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd season"`
	Kind     string    `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd episodeType"`
	Image    feedImage `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd image"`
}

type feedEntry struct {
	Title     string `xml:"title"`
	Published string `xml:"published"`
	Updated   string `xml:"updated"`
	Links     []struct {
		Rel  string `xml:"rel,attr"`
		Href string `xml:"href,attr"`
		Type string `xml:"type,attr"`
	} `xml:"link"`
	Author feedPerson `xml:"author"`
}

// ErrNotAFeed means the address isn't an RSS or Atom feed.
var ErrNotAFeed = errors.New("not a feed")

// ParseFeed reads an RSS or Atom feed as a newest-first list. Entries are enclosures when
// present, else linked pages; address resolves relative links.
func ParseFeed(data []byte, address string) (Playlist, error) {
	var doc feedDoc
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	dec.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
	if err := dec.Decode(&doc); err != nil {
		return Playlist{}, ErrNotAList
	}
	base, _ := url.Parse(address)
	resolve := func(link string) string {
		link = strings.TrimSpace(link)
		if link == "" || base == nil {
			return link
		}
		if u, err := base.Parse(link); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
			return u.String()
		}
		return ""
	}
	p := Playlist{Page: address, Current: -1, NewestFirst: true, Feed: address}
	switch strings.ToLower(doc.XMLName.Local) {
	case "rss":
		ch := doc.Channel
		p.Title = clean(ch.Title)
		show := p.Title
		author := clean(ch.Author)
		for _, it := range ch.Items {
			link := resolve(it.Enclosure.URL)
			if link != "" {
				p.Podcast = true
			} else {
				link = resolve(it.Link)
			}
			if link == "" {
				continue
			}
			tags := meta.Tags{
				Title: clean(it.Title), Album: show, Show: show, Artist: orElse(author, show),
				Date: feedDate(it.PubDate), Cover: resolve(orElse(it.Image.Href, ch.Image.Href)),
			}
			tags.Episode, _ = strconv.Atoi(strings.TrimSpace(it.Episode))
			tags.Season, _ = strconv.Atoi(strings.TrimSpace(it.Season))
			tags.Track = tags.Episode
			// Enclosure files carry no useful metadata (names are often tokens), so the feed's
			// tags override; linked pages know their own and get them only as defaults.
			label := meta.Label{Defaults: tags}
			if it.Enclosure.URL != "" {
				label = meta.Label{Tags: tags}
			}
			p.Entries = append(p.Entries, Entry{
				URL: link, Title: tags.Title, Duration: feedDuration(it.Duration),
				Meta: label, ID: strings.TrimSpace(it.GUID),
			})
			if len(p.Entries) == maxFeedEntries {
				break
			}
		}
	case "feed":
		p.Title = strings.TrimSpace(strings.TrimPrefix(clean(doc.Title), "Uploads from "))
		for _, e := range doc.Entries {
			link := ""
			for _, l := range e.Links {
				if l.Rel == "enclosure" {
					link = resolve(l.Href)
					p.Podcast = true
					break
				}
			}
			for _, l := range e.Links {
				if link == "" && (l.Rel == "" || l.Rel == "alternate") {
					link = resolve(l.Href)
				}
			}
			if link == "" {
				continue
			}
			title := clean(e.Title)
			tags := meta.Tags{Title: title, Date: feedDate(orElse(e.Published, e.Updated))}
			p.Entries = append(p.Entries, Entry{URL: link, Title: title, Meta: meta.Label{Defaults: tags}})
			if len(p.Entries) == maxFeedEntries {
				break
			}
		}
	default:
		return Playlist{}, ErrNotAList
	}
	if len(p.Entries) == 0 {
		return Playlist{}, ErrNotAList
	}
	if p.Title == "" {
		p.Title = "Feed"
	}
	return p, nil
}

func orElse(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func clean(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// feedDate normalizes RSS/Atom dates, including common malformed ones, to "2006-01-02"; "" if
// unparseable.
func feedDate(s string) string {
	s = strings.TrimSpace(s)
	for _, layout := range []string{
		time.RFC1123Z, time.RFC1123, time.RFC3339, "Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST", "2 Jan 2006 15:04:05 -0700", "Mon, 02 Jan 2006 15:04 -0700",
		"2006-01-02T15:04:05Z07:00", "2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}

// feedDuration reads itunes:duration: seconds, or "1:02:03" / "02:03".
func feedDuration(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	total := 0.0
	for part := range strings.SplitSeq(s, ":") {
		n, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return 0
		}
		total = total*60 + n
	}
	return total
}
