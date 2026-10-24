package protocol

import (
	"encoding/json"
	"time"

	"github.com/Roham1299/snag/internal/clip"
	"github.com/Roham1299/snag/internal/config"
	"github.com/Roham1299/snag/internal/cookies"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/links"
	"github.com/Roham1299/snag/internal/playlist"

	"github.com/Roham1299/snag/internal/spotify"
)

// request is any request; each type uses the fields it needs (see PROTOCOL.md).
type request struct {
	Type string          `json:"type"`
	RID  json.RawMessage `json:"rid,omitempty"`

	ID         string                     `json:"id"`
	URL        string                     `json:"url"`
	Kind       string                     `json:"kind"`
	Page       string                     `json:"page"`
	Title      string                     `json:"title"`
	Label      string                     `json:"label"`
	Filename   string                     `json:"filename"`
	Directory  string                     `json:"directory"`
	Format     string                     `json:"format"`
	Sort       string                     `json:"sort"`
	Audio      string                     `json:"audio"`
	Subs       string                     `json:"subs"`
	Sidecar    bool                       `json:"sidecar"`
	SubAuto    bool                       `json:"sub_auto"`
	SubURL     string                     `json:"sub_url"`
	SubText    string                     `json:"sub_text"`
	ClipFrom   float64                    `json:"clip_from"`
	ClipTo     float64                    `json:"clip_to"`
	Role       string                     `json:"role"`
	Size       float64                    `json:"size"`
	Headers    map[string]string          `json:"headers"`
	Cookies    []cookies.Cookie           `json:"cookies"`
	Start      *bool                      `json:"start"`
	Ask        *bool                      `json:"ask"`
	Stream     bool                       `json:"stream"`
	DeleteFile bool                       `json:"delete_file"`
	Set        map[string]json.RawMessage `json:"set"`
	Items      []haveItem                 `json:"items"`
	// Source is "page" or "selection".
	Links         []links.Found `json:"links"`
	Source        string        `json:"source"`
	Checksum      string        `json:"checksum"`
	ChecksumFiles []string      `json:"checksum_files"`
	// TMDB is the copy of TMDB's API the page loaded its show from; Open false only reports
	// whether there is a list.
	TMDB      string           `json:"tmdb"`
	Open      *bool            `json:"open"`
	Subscribe bool             `json:"subscribe"`
	Spotify   *spotify.Session `json:"spotify"`
	Feeds     []string         `json:"feeds"`
	// FeedsOnly means the page itself was already looked at.
	FeedsOnly bool   `json:"feeds_only"`
	Token     string `json:"token"`
	Error     string `json:"error"`
}

// haveItem is one quality or file the card is asking whether Snag already has.
type haveItem struct {
	URL      string  `json:"url"`
	Label    string  `json:"label"`
	Filename string  `json:"filename"`
	Format   string  `json:"format"`
	Audio    string  `json:"audio"`
	ClipFrom float64 `json:"clip_from"`
	ClipTo   float64 `json:"clip_to"`
}

func (r request) clip() clip.Range   { return clip.Range{From: r.ClipFrom, To: r.ClipTo} }
func (it haveItem) clip() clip.Range { return clip.Range{From: it.ClipFrom, To: it.ClipTo} }

// The extension calls the stream kind by the program that downloads it.
var wireKinds = map[engine.Kind]string{engine.KindHTTP: "http", engine.KindStream: "ytdlp"}

func kindFromWire(kind string) (engine.Kind, bool) {
	for k, wire := range wireKinds {
		if wire == kind {
			return k, true
		}
	}
	return "", kind == ""
}

// download is a download as the extension sees it.
type download struct {
	ID          string  `json:"id"`
	URL         string  `json:"url"`
	Page        string  `json:"page"`
	Title       string  `json:"title"`
	Kind        string  `json:"kind"`
	Label       string  `json:"label"`
	Status      string  `json:"status"`
	Path        string  `json:"path"`
	Name        string  `json:"name"`
	Size        int64   `json:"size"`
	Estimated   bool    `json:"estimated"`
	Done        int64   `json:"done"`
	Speed       int64   `json:"speed"`
	ETA         *int64  `json:"eta"` // seconds left; null when unknown
	Error       string  `json:"error"`
	Created     float64 `json:"created"`  // seconds since 1970
	Finished    float64 `json:"finished"` // seconds since 1970; 0 until it finishes
	Connections int     `json:"connections"`
	Resumable   bool    `json:"resumable"`
	Check       *check  `json:"check"`
}

// check is a finished file's checksum state (see engine.Check).
type check struct {
	Algo   string `json:"algo"`
	From   string `json:"from"`
	Result string `json:"result"` // "", "looking", "checking", "match", "mismatch" or "failed"
}

func toWire(d engine.Download) download {
	w := download{
		ID: d.ID, URL: d.URL, Page: d.Page, Title: d.Title, Kind: wireKinds[d.Kind], Label: d.Label,
		Status: string(d.Status), Path: d.Path, Name: d.Name(), Size: d.Size, Estimated: d.Estimated,
		Done: d.Done, Speed: d.Speed, Error: d.Error, Created: epoch(d.Created), Finished: epoch(d.Finished),
		Connections: d.Connections, Resumable: d.Resumable,
	}
	if d.Speed > 0 && d.Size > d.Done {
		eta := (d.Size - d.Done) / d.Speed
		w.ETA = &eta
	}
	if c := d.Check; c.Found() {
		w.Check = &check{Algo: string(c.Algo), From: c.From, Result: string(c.Result)}
	}
	return w
}

func toWireList(list []engine.Download) []download {
	out := make([]download, len(list))
	for i, d := range list {
		out[i] = toWire(d)
	}
	return out
}

func epoch(t time.Time) float64 {
	if t.IsZero() {
		return 0
	}
	return float64(t.UnixMilli()) / 1000
}

// settings are the user's settings as the protocol names them.
type settings struct {
	DownloadDir string   `json:"download_dir"`
	Connections int      `json:"connections"`
	MaxActive   int      `json:"max_active"`
	Notify      bool     `json:"notify"`
	Ask         bool     `json:"ask"`
	RecentDirs  []string `json:"recent_dirs"`
}

func settingsToWire(c config.Config) settings {
	recent := c.RecentDirs
	if recent == nil {
		recent = []string{}
	}
	return settings{c.DownloadDir, c.Connections, c.MaxActive, c.Notify, c.Ask, recent}
}

// linksFound is a set of links as the extension hears of it; the window shows the rest.
type linksFound struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

// playlistFound is a list as the extension hears of it; the window shows the rest.
type playlistFound struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Count int    `json:"count"`
}

func playlistToWire(p playlist.Playlist) playlistFound {
	return playlistFound{ID: p.ID, Title: p.Title, Count: len(p.Entries)}
}
