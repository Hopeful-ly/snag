// Package engine keeps the download list and runs downloads: queueing, pausing, resuming,
// removing and saving. The runners that move the bytes are plugged in.
package engine

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Roham1299/snag/internal/ratelimit"
	"github.com/Roham1299/snag/internal/watchlink"
)

// Errors the engine's methods return.
var (
	ErrNotFound   = errors.New("no such download")
	ErrNotPending = errors.New("download is not waiting for confirmation")
	ErrClosed     = errors.New("engine is shut down")
)

const (
	updateInterval = 250 * time.Millisecond // at most this often per download, to subscribers
	saveInterval   = 3 * time.Second        // at most this often per download, for progress alone
	removeTimeout  = 10 * time.Second
)

// Settings are the parts of the user's settings the engine follows.
type Settings struct {
	DownloadDir string
	Connections int
	MaxActive   int // Main's downloads at once, unless Queues says
	Ask         bool
	Queues      []QueueSettings
}

// Store saves and loads the download list.
type Store interface {
	Load(list *[]Download) error
	Save(list []Download) error
}

// Options are what New needs.
type Options struct {
	Store    Store
	Runners  map[Kind]Runner
	WorkRoot string // unfinished downloads get a folder each in here
	Settings Settings
	Logger   *slog.Logger // nil for slog.Default()
	// SameDrive reports whether two folders are on one drive; nil treats all as on WorkRoot's.
	SameDrive func(a, b string) bool
	// Hide hides the parts folder kept on another drive from the file manager.
	Hide func(path string) error
	// Mark marks a finished file as downloaded from the internet, from source (its address) and
	// page, as browsers do; nil marks nothing.
	Mark func(path, source, page string) error
	// OnQueue receives queue events one at a time; may be nil.
	OnQueue func(QueueEvent)
	// FindChecksum looks online for a finished file's checksum; nil looks nowhere.
	FindChecksum ChecksumFinder
}

// Engine is the download list and everything running on it. It is safe for concurrent use.
type Engine struct {
	store     Store
	runners   map[Kind]Runner
	workRoot  string
	sameDrive func(a, b string) bool
	hide      func(path string) error
	mark      func(path, source, page string) error
	log       *slog.Logger

	findChecksum ChecksumFinder

	mu          sync.Mutex
	settings    Settings
	entries     []*entry // newest first
	subscribers map[chan Event]struct{}
	closed      bool

	running      map[string]bool
	worked       map[string]bool // the running queues that have started something since they started
	lastPosition int64
	queueEvents  chan QueueEvent
	limiters     map[string]*ratelimit.Limiter

	saveMu    sync.Mutex
	dirty     chan struct{}
	saverDone chan struct{}
}

// entry is a download and what the engine tracks while it runs.
type entry struct {
	d           Download
	cancel      context.CancelFunc // set while a run is going
	stopped     chan struct{}      // closed once that run has ended
	removedFrom Status
	requeue     bool     // its run is being stopped for it to wait in line again, not paused
	readable    Readable // the running download's file, while it can be watched
	lastEvent   time.Time
	lastSave    time.Time
	fails       int         // failures since it last ran at someone's asking
	retry       *time.Timer // waiting to try a failed download again; nil when it isn't
	// checkCancel stops the checksum lookup or check; nil when neither runs.
	checkCancel context.CancelFunc
	// said is recent activity, oldest first; memory only.
	said []Said
}

// New loads the saved list and starts the downloads that were queued or running.
func New(opts Options) *Engine {
	e := &Engine{
		store:        opts.Store,
		runners:      opts.Runners,
		workRoot:     opts.WorkRoot,
		sameDrive:    opts.SameDrive,
		hide:         opts.Hide,
		mark:         opts.Mark,
		findChecksum: opts.FindChecksum,
		log:          opts.Logger,
		settings:     opts.Settings,
		subscribers:  map[chan Event]struct{}{},
		running:      map[string]bool{MainQueue: true},
		worked:       map[string]bool{},
		limiters:     map[string]*ratelimit.Limiter{},
		dirty:        make(chan struct{}, 1),
		saverDone:    make(chan struct{}),
	}
	if e.log == nil {
		e.log = slog.Default()
	}
	if opts.OnQueue != nil {
		e.queueEvents = make(chan QueueEvent, 64)
		go e.deliverQueueEvents(opts.OnQueue)
	}
	e.load()
	e.sweepWork()
	go e.saveLoop()

	e.mu.Lock()
	e.schedule()
	e.mu.Unlock()
	return e
}

func (e *Engine) load() {
	var list []Download
	if err := e.store.Load(&list); err != nil {
		e.log.Warn("couldn't load the download list", "err", err)
	}
	for _, d := range list {
		switch d.Status {
		case StatusDownloading:
			d.Status, d.Note = StatusQueued, "" // it was running when Snag stopped: carry on
		case StatusPausing:
			d.Status = StatusPaused
		case StatusRemoving:
			e.discardFiles(d) // Snag stopped halfway through removing it: finish
			continue
		}
		if r := d.Check.Result; r == CheckLooking || r == CheckChecking {
			d.Check.Result, d.Check.Read = CheckNone, 0 // cut short by quitting: asked for again
		}
		if d.Queue == "" {
			d.Queue = MainQueue // saved before there were queues
		}
		e.lastPosition = max(e.lastPosition, d.Position)
		e.entries = append(e.entries, &entry{d: d})
	}
	// Lists saved before there were queues have no places in line: oldest first, as they ran.
	for i := len(e.entries) - 1; i >= 0; i-- {
		if d := &e.entries[i].d; d.Position == 0 {
			d.Position = e.nextPosition()
		}
	}
}

// List returns every download, newest first.
func (e *Engine) List() []Download {
	e.mu.Lock()
	defer e.mu.Unlock()
	list := make([]Download, len(e.entries))
	for i, en := range e.entries {
		list[i] = en.d
	}
	return list
}

// Get returns one download.
func (e *Engine) Get(id string) (Download, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	en := e.find(id)
	if en == nil {
		return Download{}, ErrNotFound
	}
	return en.d, nil
}

// SetSettings changes the settings; downloads already running keep their connections.
func (e *Engine) SetSettings(s Settings) {
	e.mu.Lock()
	defer e.mu.Unlock()
	old := e.settings
	e.settings = s
	// Speed limits change for downloads already running too, and fewer at once stops some.
	for id, l := range e.limiters {
		l.SetRate(e.queueSettings(id).SpeedLimit)
	}
	e.trimToLimits(old)
	e.schedule()
}

// Shutdown stops running downloads so the next start resumes them, and saves the list.
func (e *Engine) Shutdown(ctx context.Context) {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.closed = true
	if e.queueEvents != nil {
		close(e.queueEvents)
	}
	// Every run is waited for, a pausing one too: its runner may still be writing.
	running := map[string]chan struct{}{}
	interrupted := map[*entry]bool{}
	for _, en := range e.entries {
		e.cancelRetry(en)
		e.stopChecking(en)
		if en.cancel != nil {
			running[en.d.ID] = en.stopped
			interrupted[en] = en.d.Status == StatusDownloading
			en.cancel()
		}
	}
	e.mu.Unlock()

	for id, stopped := range running {
		select {
		case <-stopped:
		case <-ctx.Done():
			e.log.Warn("download didn't stop in time", "id", id)
		}
	}

	e.mu.Lock()
	for en, wasDownloading := range interrupted {
		if wasDownloading && en.d.Status == StatusPaused {
			en.d.Status = StatusQueued
		}
	}
	close(e.dirty)
	e.mu.Unlock()

	<-e.saverDone
	e.saveNow()
}

// find returns the entry with the given id, or nil. The lock must be held.
func (e *Engine) find(id string) *entry {
	for _, en := range e.entries {
		if en.d.ID == id {
			return en
		}
	}
	return nil
}

// markDirty asks for the list to be saved soon. The lock must be held.
func (e *Engine) markDirty() {
	if e.closed {
		return // Shutdown saves one last time
	}
	select {
	case e.dirty <- struct{}{}:
	default:
	}
}

func (e *Engine) saveLoop() {
	defer close(e.saverDone)
	for range e.dirty {
		e.saveNow()
	}
}

func (e *Engine) saveNow() {
	e.saveMu.Lock()
	defer e.saveMu.Unlock()

	e.mu.Lock()
	list := make([]Download, len(e.entries))
	for i, en := range e.entries {
		list[i] = en.d.saved()
	}
	e.mu.Unlock()

	if err := e.store.Save(list); err != nil {
		e.log.Error("couldn't save the download list", "err", err)
	}
}

// sweepWork deletes work folders no unfinished download owns, such as one a player kept open
// (Windows won't delete files that are open) when its download finished.
func (e *Engine) sweepWork() {
	e.sweepParts()
	entries, err := os.ReadDir(e.workRoot)
	if err != nil {
		return
	}
	owned := map[string]bool{}
	for _, en := range e.entries {
		if en.d.Status != StatusDone {
			owned[en.d.Work] = true
		}
	}
	for _, entry := range entries {
		if entry.IsDir() && !owned[entry.Name()] {
			_ = os.RemoveAll(filepath.Join(e.workRoot, entry.Name()))
		}
	}
}

func (e *Engine) workDir(d Download) string {
	if d.WorkIn != "" {
		return filepath.Join(d.WorkIn, d.Work)
	}
	return filepath.Join(e.workRoot, d.Work)
}

// discardFiles deletes what a download left in its work folder and, if it never finished, the
// link at its final path into that folder. A finished file is never touched: its part file may
// still be a hard link to it.
func (e *Engine) discardFiles(d Download) {
	if d.Work == "" || filepath.Base(d.Work) != d.Work || strings.HasPrefix(d.Work, ".") {
		return // never a path that could reach outside the work root
	}
	if d.WorkIn != "" && filepath.Base(d.WorkIn) != partsFolder {
		return // nor anything but a parts folder elsewhere
	}
	work := e.workDir(d)
	if d.Path != "" && d.Finished.IsZero() {
		if err := watchlink.RemoveInto(d.Path, work); err != nil {
			e.log.Warn("couldn't delete an unfinished download's file", "path", d.Path, "err", err)
		}
	}
	if err := removeWork(work); err != nil {
		e.log.Warn("couldn't delete unfinished files", "dir", work, "err", err)
	}
	tidyParts(d)
	if d.Finished.IsZero() {
		e.restoreReplaced(d)
	}
}
