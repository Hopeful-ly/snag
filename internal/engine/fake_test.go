package engine

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/store"
)

// fakeRunner runs each download until the test finishes it or the engine cancels it.
type fakeRunner struct {
	mu      sync.Mutex
	started []Job
	results map[string]chan error // by URL
	running int
	// stopDelay is how long a run takes to stop once canceled, as yt-dlp does.
	stopDelay time.Duration
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{results: map[string]chan error{}}
}

func (f *fakeRunner) Run(ctx context.Context, job Job, report Reporter) error {
	f.mu.Lock()
	f.started = append(f.started, job)
	f.running++
	result := f.result(job.URL)
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.running--
		f.mu.Unlock()
	}()

	if job.Path == "" {
		var err error
		if job.Path, err = report.ClaimPath(cmp.Or(job.Filename, path.Base(job.URL))); err != nil {
			return err
		}
	}
	report.Readable(fakeFile{path: filepath.Join(job.WorkDir, "data.part"), have: 10})
	report.Progress(Progress{Size: 100, Done: 10, Resumable: true, Playable: 10})
	select {
	case <-ctx.Done():
		time.Sleep(f.stopDelay)
		return ctx.Err()
	case err := <-result:
		if err != nil {
			return err
		}
		report.Progress(Progress{Size: 100, Done: 100, Resumable: true})
		return os.WriteFile(job.Path, []byte("done"), 0o644)
	}
}

// fakeFile is a running download's file with its first have bytes written.
type fakeFile struct {
	path string
	have int64
}

func (f fakeFile) Path() string { return f.path }

func (f fakeFile) Have(off int64) int64 { return max(f.have-off, 0) }

func (f fakeFile) Want(int64) {}

// finish ends the run of url with err (nil for success).
func (f *fakeRunner) finish(url string, err error) {
	f.mu.Lock()
	result := f.result(url)
	f.mu.Unlock()
	result <- err
}

func (f *fakeRunner) result(url string) chan error {
	if f.results[url] == nil {
		f.results[url] = make(chan error, 1)
	}
	return f.results[url]
}

func (f *fakeRunner) jobs() []Job {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.started)
}

// waitJobs waits until Run has been called n times and returns the jobs.
func (f *fakeRunner) waitJobs(t *testing.T, n int) []Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if jobs := f.jobs(); len(jobs) >= n {
			return jobs
		}
		if time.Now().After(deadline) {
			t.Fatalf("Run was called %d times, want %d", len(f.jobs()), n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

type testEngine struct {
	*Engine
	runner   *fakeRunner
	store    *store.File[[]Download]
	dir      string // the download folder
	workRoot string
	settings Settings
	// otherDrive makes every download folder count as on another drive than the work root.
	otherDrive   atomic.Bool
	findChecksum ChecksumFinder
	// markFails makes marking finished files as downloaded fail, as on a drive that can't.
	markFails atomic.Bool

	queueMu     sync.Mutex
	queueEvents []QueueEvent
	marked      []marked
}

// marked is one finished file marked as downloaded (Options.Mark).
type marked struct{ path, source, page string }

func (te *testEngine) marks() []marked {
	te.queueMu.Lock()
	defer te.queueMu.Unlock()
	return slices.Clone(te.marked)
}

func (te *testEngine) queueHeard() []QueueEvent {
	te.queueMu.Lock()
	defer te.queueMu.Unlock()
	return slices.Clone(te.queueEvents)
}

func (te *testEngine) waitQueueEvent(t *testing.T, ev QueueEvent) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if slices.Contains(te.queueHeard(), ev) {
			return
		}
	}
	t.Fatalf("never heard %+v; heard %+v", ev, te.queueHeard())
}

func newTestEngine(t *testing.T, settings Settings) *testEngine {
	t.Helper()
	return newTestEngineWith(t, settings, nil)
}

// newTestEngineWith is newTestEngine with a ChecksumFinder.
func newTestEngineWith(t *testing.T, settings Settings, find ChecksumFinder) *testEngine {
	t.Helper()
	root := t.TempDir()
	settings.DownloadDir = filepath.Join(root, "downloads")
	if settings.MaxActive == 0 {
		settings.MaxActive = 3
	}
	te := &testEngine{
		runner:   newFakeRunner(),
		store:    store.NewFile[[]Download](filepath.Join(root, "state", "downloads.json")),
		dir:      settings.DownloadDir,
		workRoot: filepath.Join(root, "work"),
		settings: settings,

		findChecksum: find,
	}
	te.start(t)
	return te
}

// start (re)starts the engine on the same files, as when Snag starts again.
func (te *testEngine) start(t *testing.T) {
	te.Engine = New(Options{
		Store:        te.store,
		Runners:      map[Kind]Runner{KindHTTP: te.runner, KindStream: te.runner},
		WorkRoot:     te.workRoot,
		Settings:     te.settings,
		Logger:       slog.New(slog.DiscardHandler),
		SameDrive:    func(string, string) bool { return !te.otherDrive.Load() },
		FindChecksum: te.findChecksum,
		Mark: func(path, source, page string) error {
			if te.markFails.Load() {
				return errors.New("no such attribute")
			}
			te.queueMu.Lock()
			defer te.queueMu.Unlock()
			te.marked = append(te.marked, marked{path, source, page})
			return nil
		},
		OnQueue: func(ev QueueEvent) {
			te.queueMu.Lock()
			defer te.queueMu.Unlock()
			te.queueEvents = append(te.queueEvents, ev)
		},
	})
	t.Cleanup(func() { te.Shutdown(context.Background()) })
}

func (te *testEngine) add(t *testing.T, req AddRequest) Download {
	t.Helper()
	d, _, err := te.Add(req)
	if err != nil {
		t.Fatalf("Add(%s): %v", req.URL, err)
	}
	return d
}

func (te *testEngine) waitFor(t *testing.T, id string, statuses ...Status) Download {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		d, err := te.Get(id)
		if err == nil && slices.Contains(statuses, d.Status) {
			return d
		}
		if time.Now().After(deadline) {
			t.Fatalf("download %s is %q (err %v), want one of %v", id, d.Status, err, statuses)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitRunning waits until the fake runner is inside Run for the download, past claiming its path.
func (te *testEngine) waitRunning(t *testing.T, id string) Download {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		d, err := te.Get(id)
		if err == nil && d.Status == StatusDownloading && d.Done > 0 {
			return d
		}
		if time.Now().After(deadline) {
			t.Fatalf("download %s never got going: %+v, %v", id, d, err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (te *testEngine) waitGone(t *testing.T, id string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := te.Get(id); errors.Is(err, ErrNotFound) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("download %s still listed", id)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
