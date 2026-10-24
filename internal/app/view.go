package app

import (
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/Roham1299/snag/internal/config"
	"github.com/Roham1299/snag/internal/engine"
)

const (
	eventChanged   = "downloads:changed"
	eventRemoved   = "downloads:removed" // data: the id
	eventReset     = "downloads:reset"   // whole list, after a bulk add
	eventSettings  = "settings:changed"
	eventConnected = "browser:connected"
	eventQueues    = "queues:changed"  // data: the list
	eventCountdown = "queue:countdown" // data: null once over
)

func init() {
	application.RegisterEvent[Download](eventChanged)
	application.RegisterEvent[string](eventRemoved)
	application.RegisterEvent[[]Download](eventReset)
	application.RegisterEvent[config.Config](eventSettings)
	application.RegisterEvent[bool](eventConnected)
	application.RegisterEvent[[]Subscription](eventSubscriptions)
}

// Download is a download as the window shows it.
type Download struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Page      string `json:"page"`
	Title     string `json:"title"`
	Kind      string `json:"kind"` // "http" or "stream"
	Label     string `json:"label"`
	Status    string `json:"status"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Directory string `json:"directory"`
	Error     string `json:"error"`
	Note      string `json:"note"` // non-failure notice, such as saved without subtitles
	Queue     string `json:"queue"`
	Position  int64  `json:"position"` // lowest runs first
	// Subscription is "" for downloads the user added.
	Subscription string `json:"subscription"`

	Size        int64  `json:"size"` // 0 while unknown
	Estimated   bool   `json:"estimated"`
	Done        int64  `json:"done"`
	Speed       int64  `json:"speed"` // bytes/s
	ETA         *int64 `json:"eta"`   // seconds left; null when unknown
	Connections int    `json:"connections"`
	Resumable   bool   `json:"resumable"`

	Created  int64 `json:"created"`  // milliseconds since 1970
	Finished int64 `json:"finished"` // milliseconds since 1970; 0 until it finishes

	// Watch is "ready", "soon" (WatchIn estimates seconds, or null) or "" when it can't be
	// watched while downloading.
	Watch   string `json:"watch"`
	WatchIn *int64 `json:"watchIn"`

	// Where means YouTube Music had no match, so the row asks where else to look.
	Where bool `json:"where"`

	Check *Check `json:"check"`

	// Stage is post-processing such as "Fixing the video's container". Heard is when the run
	// last reported anything (ms since 1970; 0 when not running).
	Stage string `json:"stage"`
	// StageDone of StageOf is the stage's progress when known, in bytes.
	StageDone  int64 `json:"stageDone"`
	StageOf    int64 `json:"stageOf"`
	Pieces     int   `json:"pieces"`
	PiecesDone int   `json:"piecesDone"`
	Heard      int64 `json:"heard"`
}

// Check is a finished file's checksum and, once the user asks, the result of checking it.
type Check struct {
	// Algo is like "SHA-256"; Weak means it detects damage but not tampering (MD5, SHA-1).
	Algo string `json:"algo"`
	Weak bool   `json:"weak"`
	// From is "you", "page", "server", "github" or "file".
	From string `json:"from"`
	// Result is "" until checked, then "checking", "match", "mismatch" or "failed"; "looking"
	// while searching online with none known yet.
	Result string `json:"result"`
	Want   string `json:"want"`
	Got    string `json:"got"`
	Error  string `json:"error"`
	Read   int64  `json:"read"` // bytes read while checking
}

func checkOf(c engine.Check) *Check {
	if !c.Found() && c.Result != engine.CheckLooking {
		return nil
	}
	return &Check{
		Algo: c.Algo.Name(), Weak: c.Algo.Weak(), From: c.From, Result: string(c.Result),
		Want: c.Want, Got: c.Got, Error: c.Error, Read: c.Read,
	}
}

func viewOf(d engine.Download) Download {
	v := Download{
		ID: d.ID, URL: d.URL, Page: d.Page, Title: d.Title, Kind: string(d.Kind), Label: d.Label,
		Status: string(d.Status), Name: d.Name(), Path: d.Path, Directory: d.Directory, Error: d.Error, Note: d.Note,
		Queue: d.Queue, Position: d.Position, Subscription: d.Subscription,
		Size: d.Size, Estimated: d.Estimated, Done: d.Done, Speed: d.Speed,
		Connections: d.Connections, Resumable: d.Resumable, Created: d.Created.UnixMilli(),
		Where: d.Where || strings.HasSuffix(d.Error, "no match"),
	}
	if !d.Finished.IsZero() {
		v.Finished = d.Finished.UnixMilli()
	}
	if d.Speed > 0 && d.Size > d.Done {
		eta := (d.Size - d.Done) / d.Speed
		v.ETA = &eta
	}
	v.Watch, v.WatchIn = watchState(d)
	if d.Status == engine.StatusDownloading {
		v.Stage, v.Pieces, v.PiecesDone = d.Stage, d.Pieces, d.PiecesDone
		v.StageDone, v.StageOf = d.StageDone, d.StageOf
		if !d.Heard.IsZero() {
			v.Heard = d.Heard.UnixMilli()
		}
	}
	if d.Status == engine.StatusDone {
		v.Check = checkOf(d.Check)
	}
	return v
}

func viewsOf(list []engine.Download) []Download {
	out := make([]Download, len(list))
	for i, d := range list {
		out[i] = viewOf(d)
	}
	return out
}
