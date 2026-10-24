package engine

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"path"
	"path/filepath"
	"time"

	"github.com/Roham1299/snag/internal/clip"
	"github.com/Roham1299/snag/internal/cookies"
	"github.com/Roham1299/snag/internal/meta"
	"github.com/Roham1299/snag/internal/naming"
)

// Status is where a download is in its life.
type Status string

// Download statuses. Pausing and removing last until the run has actually stopped.
const (
	StatusPending     Status = "pending" // waiting for the user to confirm the folder and name
	StatusQueued      Status = "queued"
	StatusDownloading Status = "downloading"
	StatusPausing     Status = "pausing"
	StatusPaused      Status = "paused"
	StatusDone        Status = "done"
	StatusError       Status = "error"
	StatusRemoving    Status = "removing"
)

// Kind says which runner downloads something.
type Kind string

// The kinds of download.
const (
	KindHTTP   Kind = "http"
	KindStream Kind = "stream" // a video site or a streaming playlist (HLS, DASH)
)

// Download is one entry in the download list.
type Download struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Page  string `json:"page,omitempty"`
	Title string `json:"title,omitempty"`
	Kind  Kind   `json:"kind"`
	Label string `json:"label,omitempty"` // shown next to the name

	// For streams: the yt-dlp format and sort picked from the quality list, and an audio format
	// to convert to.
	Format string `json:"format,omitempty"`
	Sort   string `json:"sort,omitempty"`
	Audio  string `json:"audio,omitempty"`
	// Subs is a caption language (yt-dlp code); Sidecar writes a .srt instead of muxing. SubURL is
	// a caption file the page loaded, fetched instead of asking the site.
	Subs    string     `json:"subs,omitempty"`
	Sidecar bool       `json:"sidecar,omitempty"`
	SubAuto bool       `json:"sub_auto,omitempty"`
	SubURL  string     `json:"sub_url,omitempty"`
	SubText string     `json:"sub_text,omitempty"` // cues the player already had, as .srt
	Clip    clip.Range `json:"clip,omitzero"`      // only this part of the video; zero for all of it

	// Resolve says URL is a page whose stream only a browser finds by playing it; it is resolved
	// afresh each run, as such links expire.
	Resolve bool `json:"resolve,omitempty"`
	// Meta labels the file beyond what its source says (a song's details from Spotify); zero for
	// the source's own.
	Meta meta.Label `json:"meta,omitzero"`
	// Naming names the file from its tags per its list's template; zero for Filename's name.
	Naming naming.Naming `json:"naming,omitzero"`

	Headers map[string]string `json:"headers,omitempty"`
	Cookies []cookies.Cookie  `json:"cookies,omitempty"`

	Directory string `json:"directory"`
	Filename  string `json:"filename,omitempty"` // a name the user chose
	Path      string `json:"path,omitempty"`
	// Replace lists files the user agreed to replace (see replace.go).
	Replace []string `json:"replace,omitempty"`

	// Position is its place in its queue's line, lowest first.
	Queue    string `json:"queue"`
	Position int64  `json:"position"`
	// Subscription is the subscription that found it, kept after that is deleted; "" if the user
	// added it.
	Subscription string `json:"subscription,omitempty"`

	Status    Status `json:"status"`
	Size      int64  `json:"size"`                // 0 when unknown
	Estimated bool   `json:"estimated,omitempty"` // Size is a guess, not the server's word
	Done      int64  `json:"done"`
	Resumable bool   `json:"resumable"`
	Error     string `json:"error,omitempty"`
	// Note is information about a download that didn't fail, such as missing subtitles.
	Note string `json:"note,omitempty"`
	// Where says YouTube Music had no match, so the user is asked where else to look.
	Where         bool     `json:"where,omitempty"`
	Check         Check    `json:"check,omitzero"`
	ChecksumFiles []string `json:"checksumFiles,omitempty"`

	// Only meaningful while it runs; not saved.
	Speed       int64 `json:"speed,omitempty"` // bytes per second
	Connections int   `json:"connections,omitempty"`
	// Playable is how many bytes from the start are on disk, for watching; kept between runs.
	Playable int64 `json:"playable,omitempty"`
	ToPlay   int64 `json:"toPlay,omitempty"`
	// Stage and Pieces describe the current run (see Reporter.Stage); Heard is when it last
	// reported anything. Not saved.
	Stage      string    `json:"stage,omitempty"`
	StageDone  int64     `json:"stageDone,omitempty"`
	StageOf    int64     `json:"stageOf,omitempty"`
	Pieces     int       `json:"pieces,omitempty"`
	PiecesDone int       `json:"piecesDone,omitempty"`
	Heard      time.Time `json:"heard,omitzero"`

	Created  time.Time `json:"created"`
	Finished time.Time `json:"finished,omitzero"`

	// Work holds it until finished: under the engine's work root, or under WorkIn when set (see
	// parts.go).
	Work   string `json:"work"`
	WorkIn string `json:"workIn,omitempty"`
}

// Name is the file's name once known, else the user's choice, a name in the address, the page's
// title, or the address itself.
func (d Download) Name() string {
	if d.Path != "" {
		return filepath.Base(d.Path)
	}
	last := ""
	if u, err := url.Parse(d.URL); err == nil {
		if base := path.Base(u.Path); base != "/" && base != "." {
			last = base
		}
	}
	return cmp.Or(d.Filename, naming.FromAddress(d.URL, false), d.Title, last, d.URL)
}

// active reports whether the download still has work ahead of it.
func (d *Download) active() bool {
	switch d.Status {
	case StatusPending, StatusQueued, StatusDownloading, StatusPausing, StatusPaused:
		return true
	}
	return false
}

// saved returns the download as it is written to disk.
func (d Download) saved() Download {
	d.Speed, d.Connections = 0, 0
	d.Stage, d.Pieces, d.PiecesDone, d.Heard = "", 0, 0, time.Time{}
	d.StageDone, d.StageOf = 0, 0
	if d.Status == StatusDone {
		d.Cookies = nil // only kept while it may need to resume
	}
	return d
}

func randomHex(bytes int) string {
	b := make([]byte, bytes)
	_, _ = rand.Read(b) // never fails
	return hex.EncodeToString(b)
}
