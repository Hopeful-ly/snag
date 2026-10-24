// Package httpdl downloads plain files over HTTP, in parallel ranges when the server allows,
// resuming from state saved in the job's work folder.
package httpdl

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Roham1299/snag/internal/checksum"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/naming"
	"github.com/Roham1299/snag/internal/subs"
	"github.com/Roham1299/snag/internal/watchlink"
)

const (
	stallTimeout  = 30 * time.Second
	maxRetries    = 6
	tickInterval  = 500 * time.Millisecond
	stateInterval = 3 * time.Second
)

// Runner downloads plain files. It implements engine.Runner.
type Runner struct {
	client  *http.Client
	backoff func(try int) time.Duration
}

// New returns a Runner with its own connection pool.
func New() *Runner {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = stallTimeout
	transport.MaxIdleConnsPerHost = 32
	return &Runner{client: &http.Client{Transport: transport}, backoff: exponentialBackoff}
}

// Run downloads job's file.
func (r *Runner) Run(ctx context.Context, job engine.Job, report engine.Reporter) error {
	d := &download{runner: r, job: job, report: report, url: job.URL, path: job.Path}
	return d.run(ctx)
}

func exponentialBackoff(try int) time.Duration {
	return min(time.Second<<try, 20*time.Second)
}

type download struct {
	runner *Runner
	job    engine.Job
	report engine.Reporter

	url    string // after redirects
	path   string
	linked bool
	file   *os.File

	mu        sync.Mutex // guards the fields below and writes to file
	size      int64      // 0 while unknown
	resumable bool
	segs      segments
	done      int64
	workers   int
	limit     int // a player jumping ahead may add one connection beyond this
	sniffed   bool
	failure   error
	// launch is nil when the download isn't running.
	launch func(seg *segment, resp *http.Response)
}

func (d *download) partPath() string  { return filepath.Join(d.job.WorkDir, "data.part") }
func (d *download) statePath() string { return filepath.Join(d.job.WorkDir, "state.json") }

func (d *download) run(ctx context.Context) error {
	if err := os.MkdirAll(d.job.WorkDir, 0o700); err != nil {
		return err
	}
	first, err := d.prepare(ctx) //nolint:bodyclose // the worker that reads it closes it
	if err != nil {
		return err
	}
	err = d.fetch(ctx, first)
	if closeErr := d.file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		d.saveState()
		return err
	}
	return d.finish(ctx)
}

// prepare resumes from saved state, or opens the first connection to learn the size and name and
// returns it unread (nil when resuming).
func (d *download) prepare(ctx context.Context) (*http.Response, error) {
	if d.path != "" && d.loadState() {
		if f, err := os.OpenFile(d.partPath(), os.O_RDWR, 0); err == nil {
			d.file = f
			d.linkUp()
			d.report.Readable(d)
			d.progress(0)
			return nil, nil
		}
	}

	resp, err := d.get(ctx, 0, -1)
	if err != nil {
		return nil, err
	}
	d.url = resp.Request.URL.String()
	if sum, ok := checksum.FromHeaders(resp.Header); ok {
		d.report.Checksum(sum) // the first request covers the whole file
	}
	size, ranged := contentLength(resp)
	if d.path == "" {
		if d.path, err = d.report.ClaimPath(d.name(resp)); err != nil {
			resp.Body.Close()
			return nil, err
		}
	}
	f, err := os.OpenFile(d.partPath(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err == nil && size > 0 {
		if err = f.Truncate(size); err != nil {
			f.Close()
		}
	}
	if err != nil {
		resp.Body.Close()
		return nil, err
	}

	d.file = f
	d.size, d.resumable = size, ranged
	d.segs = newSegments(size, ranged && isMedia(d.path))
	d.linkUp()
	d.report.Readable(d)
	d.progress(0)
	return resp, nil
}

// name prefers the user's choice, then the server's, the URL's, the page title, then a generic
// one; never a URL token.
func (d *download) name(resp *http.Response) string {
	name := firstNonEmpty(
		d.job.Filename,
		naming.FromContentDisposition(resp.Header.Get("Content-Disposition")),
		naming.FromAddress(d.url, false),
		d.job.Title,
		naming.FromAddress(d.url, true),
	)
	name = naming.Safe(name, "download")
	return naming.Safe(naming.WithExtension(name, resp.Header.Get("Content-Type")), "download")
}

func (d *download) finish(ctx context.Context) error {
	if err := watchlink.Finish(d.path, d.partPath(), d.linked); err != nil {
		return err
	}
	if d.job.SubURL != "" || d.job.SubText != "" {
		if err := subs.Attach(ctx, subs.Request{
			URL:     d.job.SubURL,
			Text:    d.job.SubText,
			Video:   d.path,
			Lang:    d.job.Subs,
			Sidecar: d.job.Sidecar,
			Headers: d.job.Headers,
			Cookies: d.job.Cookies,
			WorkDir: d.job.WorkDir,
		}); err != nil {
			slog.Warn("couldn't keep the subtitle file", "download", d.job.URL, "err", err)
		}
	}
	// On Windows this fails while a player has the file open; the engine sweeps it on next start.
	if err := os.RemoveAll(d.job.WorkDir); err != nil {
		slog.Warn("couldn't delete a finished download's work folder", "dir", d.job.WorkDir, "err", err)
	}
	d.progress(0)
	return nil
}

func (d *download) progress(speed int64) {
	d.mu.Lock()
	p := engine.Progress{Size: d.size, Done: d.done, Speed: speed, Connections: d.workers, Resumable: d.resumable}
	p.Playable = d.haveLocked(0)
	d.mu.Unlock()
	d.report.Progress(p)
}

// Path is the part file being written.
func (d *download) Path() string { return d.partPath() }

// Have returns how many contiguous bytes from off are written.
func (d *download) Have(off int64) int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.haveLocked(off)
}

// Want fetches from off next, for a player waiting there.
func (d *download) Want(off int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.size == 0 || off >= d.size {
		return
	}
	half := d.segs.want(off)
	if half == nil || d.launch == nil || d.workers == 0 {
		return // a free connection takes it, or the download is ending
	}
	half.busy = true
	d.launch(half, nil) // one extra connection, until it finishes this part
}

func (d *download) haveLocked(off int64) int64 {
	n := d.segs.have(off)
	if d.size > 0 {
		n = min(n, max(d.size-off, 0))
	}
	return n
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
