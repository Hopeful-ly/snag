package engine

import (
	"cmp"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Roham1299/snag/internal/checksum"
	"github.com/Roham1299/snag/internal/clip"
	"github.com/Roham1299/snag/internal/cookies"
	"github.com/Roham1299/snag/internal/meta"
	"github.com/Roham1299/snag/internal/naming"
)

// ErrBadURL is returned for addresses Snag can't download from.
var ErrBadURL = errors.New("need an http or https address")

// ErrBadClip is returned for a clip that doesn't end after it starts, inside the video.
var ErrBadClip = errors.New("a clip has to end after it starts")

// AddRequest describes a new download. Only URL is required.
type AddRequest struct {
	URL   string
	Page  string
	Title string
	Label string
	Kind  Kind // "" to guess from the address

	Format  string
	Sort    string
	Audio   string
	Subs    string
	Sidecar bool
	SubAuto bool
	SubURL  string
	SubText string
	Clip    clip.Range

	Resolve bool
	Meta    meta.Label
	Naming  naming.Naming

	Headers map[string]string
	Cookies []cookies.Cookie

	Directory string // "" for the default download folder
	Filename  string
	Size      int64 // a size known in advance, such as from a quality list

	// Checksum is one the page shows, in any common notation; ChecksumFiles are checksum files
	// it links to.
	Checksum      string
	ChecksumFiles []string

	Paused bool
	Queue  string // "" for Main
	// Subscription is the subscription that found it; "" if the user added it.
	Subscription string
	Confirmed    bool // the folder and name are settled, so don't ask even when Settings.Ask is on
}

// Add adds a download. When the list already has it unfinished, Add returns that one with
// duplicate set instead, resuming it if paused: the same link clicked again, or the same video
// from the same page under a new link, since stream links expire.
func (e *Engine) Add(req AddRequest) (d Download, duplicate bool, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.add(req, true)
}

// AddMany adds every request as Add does but notifies subscribers once with the whole list:
// an event per download for hundreds of songs would stall the window.
func (e *Engine) AddMany(reqs []AddRequest) (added, already int, err error) {
	if len(reqs) == 0 {
		return 0, 0, nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return 0, 0, ErrClosed
	}
	for _, req := range reqs {
		_, dup, addErr := e.add(req, false)
		if addErr != nil {
			e.schedule()
			e.emitReset()
			return added, already, addErr
		}
		if dup {
			already++
		} else {
			added++
		}
	}
	e.schedule()
	e.emitReset()
	return added, already, nil
}

// add adds one download; without announce the caller (AddMany) notifies and schedules. The lock
// must be held.
func (e *Engine) add(req AddRequest, announce bool) (d Download, duplicate bool, err error) {
	u, err := url.Parse(req.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Download{}, false, ErrBadURL
	}
	if !req.Clip.Valid() {
		return Download{}, false, ErrBadClip
	}
	kind := cmp.Or(req.Kind, guessKind(u))
	if req.Clip.IsSet() {
		kind = KindStream // yt-dlp cuts the part out, of a plain file too
	}
	if e.runners[kind] == nil {
		return Download{}, false, fmt.Errorf("can't download %s yet", kind)
	}

	if e.closed {
		return Download{}, false, ErrClosed
	}
	if en := e.sameDownload(req, kind); en != nil {
		if en.d.Status == StatusPaused && !req.Paused {
			e.resume(en)
		}
		return en.d, true, nil
	}

	filename := naming.Safe(req.Filename, "")
	if kind == KindStream {
		filename = streamExtension.ReplaceAllString(filename, "") // yt-dlp picks the container
	}
	if req.Clip.IsSet() {
		video := cmp.Or(filename, naming.Safe(req.Title, ""), "Clip")
		filename = naming.Clipped(video, req.Clip)
	}
	en := &entry{d: Download{
		ID:        randomHex(6),
		URL:       req.URL,
		Page:      req.Page,
		Title:     req.Title,
		Kind:      kind,
		Label:     req.Label,
		Format:    req.Format,
		Sort:      req.Sort,
		Audio:     req.Audio,
		Subs:      req.Subs,
		Sidecar:   req.Sidecar,
		SubAuto:   req.SubAuto,
		SubURL:    req.SubURL,
		SubText:   req.SubText,
		Clip:      req.Clip,
		Resolve:   req.Resolve,
		Meta:      req.Meta,
		Naming:    req.Naming,
		Headers:   req.Headers,
		Cookies:   req.Cookies,
		Directory: cmp.Or(req.Directory, e.settings.DownloadDir),
		Filename:  filename,
		Queue:     cmp.Or(req.Queue, MainQueue),
		Position:  e.nextPosition(),
		Status:    StatusQueued,
		Size:      req.Size,
		Estimated: req.Size > 0 && kind == KindStream,
		Resumable: true,
		Created:   time.Now(),
		Work:      randomHex(16),

		ChecksumFiles: req.ChecksumFiles,
		Subscription:  req.Subscription,
	}}
	if sum, ok := checksum.Parse(req.Checksum); ok {
		en.d.Check.use(sum, FromPage)
	}
	switch {
	case req.Paused:
		en.d.Status = StatusPaused
	case e.settings.Ask && !req.Confirmed:
		en.d.Status = StatusPending
	}
	e.entries = slices.Insert(e.entries, 0, en)
	e.markDirty()
	if announce {
		e.emit(EventAdded, en.d)
		e.schedule()
	}
	return en.d, false, nil
}

// sameDownload finds an unfinished download that req asks for again. A stream that stopped
// under a link that has since expired gets req's link and carries on from where it was.
// The lock must be held.
func (e *Engine) sameDownload(req AddRequest, kind Kind) *entry {
	for _, en := range e.entries {
		d := &en.d
		if d.URL == req.URL && d.active() && d.Status != StatusPausing && d.Format == req.Format && d.Audio == req.Audio &&
			d.Subs == req.Subs && d.Sidecar == req.Sidecar && d.SubURL == req.SubURL && d.Clip == req.Clip {
			return en
		}
	}
	if kind != KindStream || req.Page == "" {
		return nil
	}
	for _, en := range e.entries {
		d := &en.d
		stopped := d.Status == StatusPaused || d.Status == StatusError || d.Status == StatusQueued
		if d.Kind == KindStream && stopped && d.URL != req.URL && d.Page == req.Page &&
			d.Title == req.Title && d.Label == req.Label && d.Format == req.Format && d.Audio == req.Audio &&
			d.Subs == req.Subs && d.Sidecar == req.Sidecar && d.Clip == req.Clip {
			e.log.Info("new link for a download", "id", d.ID)
			d.URL = req.URL
			d.SubURL = req.SubURL
			if len(req.Headers) > 0 {
				d.Headers = req.Headers
			}
			if len(req.Cookies) > 0 {
				d.Cookies = req.Cookies
			}
			if d.Status == StatusError {
				d.Status, d.Error = StatusPaused, ""
			}
			e.markDirty()
			e.changed(en, true)
			return en
		}
	}
	return nil
}

// streamExtension is a media extension on a user-given stream name; yt-dlp adds its own.
var streamExtension = regexp.MustCompile(`(?i)\.(mp4|mkv|webm|m4a|mp3|opus|ogg)$`)

// guessKind picks the runner from the address alone: streaming playlists are streams, the rest HTTP.
func guessKind(u *url.URL) Kind {
	p := strings.ToLower(u.Path)
	for _, suffix := range []string{".m3u8", ".mpd", ".f4m", ".ism/manifest"} {
		if strings.HasSuffix(p, suffix) {
			return KindStream
		}
	}
	return KindHTTP
}
