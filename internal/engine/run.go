package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Roham1299/snag/internal/checksum"
	"github.com/Roham1299/snag/internal/naming"
)

// schedule starts waiting downloads in each running queue, first in line first, up to its limit,
// then ends the runs of queues that are done. The lock must be held.
func (e *Engine) schedule() {
	if e.closed {
		return
	}
	busy := map[string]int{}
	for _, en := range e.entries {
		if en.d.Status == StatusDownloading {
			busy[e.queueOf(en.d)]++
		}
	}
	for _, en := range e.waitingInOrder() {
		queue := e.queueOf(en.d)
		if !e.running[queue] || busy[queue] >= e.queueSettings(queue).AtOnce {
			continue
		}
		if e.runners[en.d.Kind] == nil {
			en.d.Status, en.d.Error = StatusError, fmt.Sprintf("can't download %s yet", en.d.Kind)
			e.changed(en, true)
			continue
		}
		e.start(en)
		busy[queue]++
		e.worked[queue] = true
	}
	e.checkDone()
}

// start runs a download in its own goroutine. The lock must be held.
func (e *Engine) start(en *entry) {
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	e.cancelRetry(en)
	en.cancel, en.stopped = cancel, stopped
	en.d.Status, en.d.Error, en.d.Note, en.d.Speed = StatusDownloading, "", "", 0
	en.d.Where = false

	e.placeParts(&en.d)
	runner := e.runners[en.d.Kind]
	job := Job{
		URL:         en.d.URL,
		Resolve:     en.d.Resolve,
		Meta:        en.d.Meta,
		Naming:      en.d.Naming,
		Headers:     en.d.Headers,
		Cookies:     en.d.Cookies,
		Title:       en.d.Title,
		Filename:    en.d.Filename,
		Path:        en.d.Path,
		WorkDir:     e.workDir(en.d),
		Connections: e.settings.Connections,
		Format:      en.d.Format,
		Fallback:    e.queueSettings(e.queueOf(en.d)).Fallback,
		Sort:        en.d.Sort,
		Audio:       en.d.Audio,
		Subs:        en.d.Subs,
		Sidecar:     en.d.Sidecar,
		SubAuto:     en.d.SubAuto,
		SubURL:      en.d.SubURL,
		SubText:     en.d.SubText,
		Clip:        en.d.Clip,
	}
	if en.d.Estimated {
		job.SizeHint = en.d.Size
	}
	if queue := e.queueSettings(e.queueOf(en.d)); queue.SpeedLimit > 0 {
		job.Limit = e.limiter(queue)
		job.SpeedLimit = max(queue.SpeedLimit/int64(queue.AtOnce), 1)
	}
	e.log.Info("download started", "id", en.d.ID, "url", en.d.URL, "from", en.d.Done)
	en.d.Heard = time.Now()
	if en.d.Done > 0 {
		en.say("Started again, from what was saved")
	} else {
		en.say("Started")
	}
	go func() {
		defer close(stopped)
		err := runner.Run(ctx, job, &reporter{e: e, en: en})
		interrupted := ctx.Err() != nil
		cancel()
		e.finished(en, err, interrupted)
	}()
	e.changed(en, true)
}

// finished records how a run ended and starts whatever can run next.
func (e *Engine) finished(en *entry, err error, interrupted bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	requeue := en.requeue
	en.cancel, en.stopped, en.readable, en.requeue = nil, nil, nil, false
	en.d.Speed, en.d.Connections = 0, 0
	en.d.Stage, en.d.Pieces, en.d.PiecesDone = "", 0, 0
	en.d.StageDone, en.d.StageOf = 0, 0
	switch {
	case err == nil:
		en.say("Finished")
	case interrupted:
		en.say("Stopped")
	default:
		en.say("Failed: " + err.Error())
	}

	d := &en.d
	switch {
	case d.Status == StatusRemoving:
		return // finishRemove takes it from here
	case err == nil:
		en.fails = 0
		asFinished := *d // with the cookies, which looking for its checksum may need
		d.Status, d.Finished, d.Cookies = StatusDone, time.Now(), nil
		asFinished.Status, asFinished.Finished = d.Status, d.Finished
		e.lookForChecksum(en, asFinished)
		e.dropReplaced(d)
		go e.tidyFinished(*d)
		if d.Size == 0 {
			d.Size = d.Done
		}
		e.log.Info("download finished", "id", d.ID, "path", d.Path)
	case interrupted && requeue:
		// Its queue stopped, or it moved to one that doesn't run: it waits in line again.
		if d.Status == StatusPausing {
			d.Status = StatusQueued
		}
	case interrupted:
		// Paused or shut down. A download resumed while it was stopping stays queued and runs again.
		if d.Status == StatusDownloading || d.Status == StatusPausing {
			d.Status = StatusPaused
		}
	default:
		var ask *NeedsPlace
		if errors.As(err, &ask) {
			// The same search would miss again, so it waits for the user to name a place.
			en.fails = 0
			d.Status, d.Error, d.Note, d.Where = StatusError, ask.Error(), "", true
			e.log.Info("no match; asking where else to look", "id", d.ID, "url", d.URL)
		} else if !e.armRetry(en, err) {
			en.fails = 0
			d.Status, d.Error, d.Note = StatusError, err.Error(), ""
			e.log.Info("download failed", "id", d.ID, "url", d.URL, "err", err)
		}
	}
	e.markDirty()
	e.changed(en, true)
	e.schedule()
}

// NeedsPlace is a song YouTube Music doesn't have. The download asks where else to look
// instead of trying the same search again. Text is what its row shows.
type NeedsPlace struct{ Text string }

func (e *NeedsPlace) Error() string {
	if e.Text != "" {
		return e.Text
	}
	return "no match"
}

// armRetry tries a failed download again when its queue still has tries left. It keeps the
// download's place, so the next one waits. The lock must be held.
func (e *Engine) armRetry(en *entry, err error) bool {
	q := e.queueSettings(e.queueOf(en.d))
	if en.fails >= q.Retries || q.Retries <= 0 {
		return false
	}
	en.fails++
	wait := q.RetryWait
	en.d.Status, en.d.Error, en.d.Speed = StatusDownloading, "", 0
	en.d.Note = retryNote(en.fails, q.Retries, wait)
	e.log.Info("download failed; trying again", "id", en.d.ID, "try", en.fails, "of", q.Retries, "err", err)
	if wait <= 0 {
		e.start(en)
		return true
	}
	en.retry = time.AfterFunc(wait, func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.closed || en.retry == nil || en.d.Status != StatusDownloading || en.cancel != nil {
			return
		}
		en.retry = nil
		if !e.running[e.queueOf(en.d)] {
			en.d.Status, en.d.Note = StatusQueued, ""
			e.markDirty()
			e.changed(en, true)
			return
		}
		e.start(en)
	})
	return true
}

// cancelRetry stops a wait to try a download again, if there is one. The lock must be held.
func (e *Engine) cancelRetry(en *entry) {
	if en.retry == nil {
		return
	}
	en.retry.Stop()
	en.retry = nil
}

// retryNote says a failed download will be tried again, and which try this is.
func retryNote(try, of int, wait time.Duration) string {
	when := "straight away"
	switch s := int(wait.Round(time.Second) / time.Second); {
	case s == 1:
		when = "in 1 second"
	case s > 1:
		when = fmt.Sprintf("in %d seconds", s)
	}
	return fmt.Sprintf("Trying again %s (%d of %d)", when, try, of)
}

// pathInUse reports whether an unfinished download already has path. The lock must be held.
func (e *Engine) pathInUse(path string) bool {
	for _, en := range e.entries {
		if en.d.Path == path && en.d.Status != StatusDone {
			return true
		}
	}
	return false
}

// reporter is the Reporter for one run of one download.
type reporter struct {
	e  *Engine
	en *entry
}

func (r *reporter) ClaimPath(name string) (string, error) {
	r.e.mu.Lock()
	dir := r.en.d.Directory
	r.e.mu.Unlock()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	r.e.mu.Lock()
	defer r.e.mu.Unlock()
	if r.en.d.Path == "" {
		r.e.setAsideReplaced(&r.en.d) // the first claim of its life: make room
	}
	inUse := func(path string) bool { return path != r.en.d.Path && r.e.pathInUse(path) }
	path := naming.UniquePath(dir, naming.Safe(name, "download"), inUse)
	r.en.d.Path = path
	r.e.markDirty()
	r.e.changed(r.en, true)
	return path, nil
}

func (r *reporter) Readable(file Readable) {
	r.e.mu.Lock()
	defer r.e.mu.Unlock()
	r.en.readable = file
}

func (r *reporter) Note(text string) {
	r.e.mu.Lock()
	defer r.e.mu.Unlock()
	r.en.d.Note = text
	r.e.markDirty()
	r.e.changed(r.en, true)
}

func (r *reporter) Checksum(sum checksum.Sum) {
	r.e.mu.Lock()
	defer r.e.mu.Unlock()
	if !r.en.d.Check.Found() && !sum.IsZero() {
		r.en.d.Check.use(sum, FromServer)
		r.e.markDirty()
	}
}

func (r *reporter) Titled(title string) {
	r.e.mu.Lock()
	defer r.e.mu.Unlock()
	if r.en.d.Title == "" && title != "" {
		r.en.d.Title = title
		r.e.markDirty()
		r.e.changed(r.en, true)
	}
}

func (r *reporter) Stage(text string) {
	r.e.mu.Lock()
	defer r.e.mu.Unlock()
	d := &r.en.d
	d.Heard = time.Now()
	if d.Stage == text {
		return
	}
	d.Stage, d.StageDone, d.StageOf = text, 0, 0
	if text != "" {
		d.Speed = 0 // nothing is being fetched now
		r.en.say(text + "…")
	}
	r.e.changed(r.en, true)
}

func (r *reporter) StageProgress(done, of int64) {
	r.e.mu.Lock()
	defer r.e.mu.Unlock()
	d := &r.en.d
	if d.Stage == "" {
		return
	}
	d.StageDone, d.StageOf, d.Heard = done, of, time.Now()
	r.e.changed(r.en, false)
}

func (r *reporter) Said(text string) {
	r.e.mu.Lock()
	defer r.e.mu.Unlock()
	r.en.d.Heard = time.Now()
	r.en.say(text)
}

func (r *reporter) Progress(p Progress) {
	r.e.mu.Lock()
	defer r.e.mu.Unlock()
	d := &r.en.d
	d.Size, d.Done, d.Speed, d.Connections = p.Size, p.Done, p.Speed, p.Connections
	d.Resumable, d.Estimated, d.Playable, d.ToPlay = p.Resumable, p.Estimated, p.Playable, p.ToPlay
	d.Pieces, d.PiecesDone, d.Heard = p.Pieces, p.PiecesDone, time.Now()
	if now := time.Now(); now.Sub(r.en.lastSave) >= saveInterval {
		r.en.lastSave = now
		r.e.markDirty()
	}
	r.e.changed(r.en, false)
}
