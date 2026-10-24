// Package probe finds out what a page or stream offers (title, qualities, sizes) before download.
package probe

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Roham1299/snag/internal/cookies"
	"github.com/Roham1299/snag/internal/tools"
	"github.com/Roham1299/snag/internal/ytdlp"
)

const (
	cacheFor        = 10 * time.Minute
	cacheSize       = 200
	maxEstimates    = 4 // stream variants whose size is estimated
	estimateTimeout = 20 * time.Second
)

// Request asks what an address offers.
type Request struct {
	URL     string
	Headers map[string]string
	Cookies []cookies.Cookie
	Stream  bool
	// For List: Fresh bypasses the cache; Max limits entries (0 for all, up to ytdlp.MaxEntries).
	Fresh bool
	Max   int
}

// Result is what an address offers.
type Result struct {
	Supported bool   `json:"supported"`
	Reason    string `json:"reason,omitempty"` // "" means "nothing to offer"

	Title     string   `json:"title,omitempty"`
	Thumbnail string   `json:"thumbnail,omitempty"`
	Duration  float64  `json:"duration,omitempty"` // seconds
	Site      string   `json:"site,omitempty"`
	URL       string   `json:"url,omitempty"`
	Choices   []Choice `json:"choices,omitempty"`
	// Variants are a master playlist's media playlists, so the extension can hide them.
	Variants  []string   `json:"variants,omitempty"`
	Subtitles []Subtitle `json:"subtitles,omitempty"`
	// Playlist is set only when it has more than one entry.
	Playlist *Playlist `json:"playlist,omitempty"`
}

// Playlist names the list a page is in or is.
type Playlist struct {
	Title string `json:"title"`
	Count int    `json:"count"`
}

// Prober answers what addresses offer, caching answers for a while.
type Prober struct {
	tools  tools.Paths
	client *http.Client

	results cache[Result]
	lists   cache[ytdlp.Info]
}

// New returns a Prober that runs the given programs.
func New(t tools.Paths) *Prober {
	return &Prober{tools: t, client: &http.Client{Timeout: 15 * time.Second}}
}

// Probe returns what req.URL offers. Failures are in the result's Reason.
func (p *Prober) Probe(ctx context.Context, req Request) Result {
	stream := req.Stream || isPlaylistAddress(req.URL)
	key := req.URL
	if stream {
		key = "stream:" + key
	}
	if result, ok := p.results.get(key); ok {
		return result
	}
	if p.tools.YtDLP == "" {
		return Result{Reason: "yt-dlp is missing"}
	}

	var info ytdlp.Info
	var list *Playlist
	var err error
	if stream {
		info, err = p.describe(ctx, req, true)
	} else {
		info, list, err = p.describePage(ctx, req)
	}
	if errors.Is(err, ytdlp.ErrUnsupported) {
		result := Result{}
		p.results.put(key, result)
		return result
	}
	if err != nil {
		return Result{Reason: truncate(err.Error(), 300)}
	}

	result := Result{
		Supported: true,
		Title:     info.Title,
		Thumbnail: info.Thumbnail,
		Duration:  info.Duration,
		Site:      info.Site,
		URL:       req.URL,
		Playlist:  list,
	}
	result.Subtitles = subtitlesOf(info)
	if stream {
		result.Choices = streamChoices(info, req.URL)
		p.estimateSizes(ctx, result.Choices, req)
	} else {
		result.Choices = pageChoices(info)
		if info.WebpageURL != "" {
			result.URL = info.WebpageURL
		}
	}
	for _, f := range info.Formats {
		if f.URL != "" {
			result.Variants = append(result.Variants, f.URL)
		}
	}
	p.results.put(key, result)
	return result
}

// List describes the list a page is in or is, entries by address and title only (see
// ytdlp.List). A page in no list is described in full.
func (p *Prober) List(ctx context.Context, req Request) (ytdlp.Info, error) {
	if list, ok := p.lists.get(req.URL); ok && !req.Fresh && req.Max == 0 {
		return list, nil
	}
	if p.tools.YtDLP == "" {
		return ytdlp.Info{}, errors.New("yt-dlp is missing")
	}
	cookieFile, remove, err := cookies.TempFile(req.Cookies)
	if err != nil {
		return ytdlp.Info{}, err
	}
	defer remove()
	opts := p.infoOptions(req, false, cookieFile)
	opts.Max = req.Max
	list, err := ytdlp.List(ctx, p.tools.YtDLP, opts)
	if err != nil {
		return ytdlp.Info{}, err
	}
	if req.Max == 0 {
		p.lists.put(req.URL, list) // a truncated list is no answer for the whole
	}
	return list, nil
}

// describePage lists first: for a single video that is already the full description, so only a
// page in a list costs a second run. A list page stands for its first video.
func (p *Prober) describePage(ctx context.Context, req Request) (ytdlp.Info, *Playlist, error) {
	list, err := p.List(ctx, req)
	if err != nil && inList(req.URL) && ctx.Err() == nil {
		// The list may be private or gone; the video is still a video.
		info, err := p.describe(ctx, req, false)
		return info, nil, err
	}
	if err != nil || list.Type != "playlist" {
		return list, nil, err
	}
	if len(list.Entries) == 0 {
		return ytdlp.Info{}, nil, errors.New("empty playlist")
	}
	video := req
	if list.EntryFor(req.URL) < 0 && list.Entries[0].URL != "" {
		video.URL = list.Entries[0].URL
	}
	info, err := p.describe(ctx, video, false)
	if err != nil || !list.IsList() {
		return info, nil, err
	}
	return info, &Playlist{Title: list.Title, Count: len(list.Entries)}, nil
}

// inList reports whether a page names a list besides its video (YouTube's watch?v=…&list=…).
func inList(page string) bool {
	u, err := url.Parse(page)
	return err == nil && u.Query().Get("list") != ""
}

func (p *Prober) describe(ctx context.Context, req Request, stream bool) (ytdlp.Info, error) {
	cookieFile, remove, err := cookies.TempFile(req.Cookies)
	if err != nil {
		return ytdlp.Info{}, err
	}
	defer remove()
	return ytdlp.Describe(ctx, p.tools.YtDLP, p.infoOptions(req, stream, cookieFile))
}

func (p *Prober) infoOptions(req Request, stream bool, cookieFile string) ytdlp.InfoOptions {
	return ytdlp.InfoOptions{
		URL:        req.URL,
		Stream:     stream,
		Headers:    req.Headers,
		CookieFile: cookieFile,
		Programs:   ytdlp.Programs{FFmpeg: p.tools.FFmpeg, Node: p.tools.Node},
	}
}

func (p *Prober) estimateSizes(ctx context.Context, choices []Choice, req Request) {
	ctx, cancel := context.WithTimeout(ctx, estimateTimeout)
	defer cancel()
	var wg sync.WaitGroup
	started := 0
	for i := range choices {
		c := &choices[i]
		if c.Size > 0 || c.playlist == "" || c.AudioOnly || started == maxEstimates {
			continue
		}
		started++
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Size = p.estimateHLSSize(ctx, c.playlist, req.Headers, req.Cookies)
		}()
	}
	wg.Wait()
}

var (
	streamAddress = regexp.MustCompile(`(?i)\.(m3u8|mpd|f4m|ism/manifest)$`)
	fileAddress   = regexp.MustCompile(`(?i)\.(mp4|webm|mkv|mov|avi|flv|m4v|mp3|m4a|ogg|opus|flac|wav|zip|rar|7z|tar|gz|xz|zst|iso|exe|msi|deb|rpm|apk|pdf|dmg|appimage|bin|img)$`)
)

// IsStream reports whether an address should be downloaded with yt-dlp: a streaming playlist or
// a page yt-dlp knows. File extensions decide when they can; anything else is probed.
func (p *Prober) IsStream(ctx context.Context, address string) bool {
	u, err := url.Parse(address)
	if err != nil {
		return false
	}
	switch path := strings.ToLower(u.Path); {
	case streamAddress.MatchString(path):
		return true
	case fileAddress.MatchString(path):
		return false
	}
	return p.Probe(ctx, Request{URL: address}).Supported
}

func isPlaylistAddress(address string) bool {
	u, err := url.Parse(address)
	if err != nil {
		return false
	}
	path := strings.ToLower(u.Path)
	return strings.HasSuffix(path, ".m3u8") || strings.HasSuffix(path, ".mpd")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
