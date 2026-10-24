package engine

import (
	"cmp"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Roham1299/snag/internal/naming"
)

// ConfirmRequest is the user's answer for a pending download.
type ConfirmRequest struct {
	Directory string // "" to keep the default folder
	Filename  string // "" to name it from the server or the page
	Paused    bool
	// Replace overwrites an existing file of that name; otherwise it is saved as "name (1)".
	Replace bool
	Queue   string // at the end of its line; "" keeps its queue
}

// Confirm settles where a pending download goes and queues it.
func (e *Engine) Confirm(id string, req ConfirmRequest) error {
	e.mu.Lock()
	en := e.find(id)
	if en == nil {
		e.mu.Unlock()
		return ErrNotFound
	}
	dir := cmp.Or(req.Directory, en.d.Directory)
	e.mu.Unlock()

	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if err := checkWritable(dir); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if en.d.Status != StatusPending {
		return ErrNotPending
	}
	en.d.Replace = nil
	if req.Replace {
		paths, inUse, _ := e.clashes(en, dir, req.Filename)
		if inUse {
			return ErrNameInUse
		}
		en.d.Replace = paths
	}
	en.d.Directory = dir
	if name := naming.Safe(req.Filename, ""); name != "" {
		if en.d.Kind == KindStream {
			name = streamExtension.ReplaceAllString(name, "")
		}
		en.d.Filename = name
	}
	if req.Queue != "" && req.Queue != en.d.Queue {
		en.d.Queue, en.d.Position = req.Queue, e.nextPosition()
	}
	en.d.Status = StatusQueued
	if req.Paused {
		en.d.Status = StatusPaused
	}
	e.markDirty()
	e.changed(en, true)
	e.schedule()
	return nil
}

// UseFound queues a download YouTube Music couldn't match at address, a video the user picked.
// Its page stays the original song's, so the file keeps that song's details.
func (e *Engine) UseFound(id, address string) error {
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("that doesn't look like a link (it should start with http:// or https://)")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	en := e.find(id)
	if en == nil {
		return ErrNotFound
	}
	if en.d.Status != StatusError || (!en.d.Where && !strings.HasSuffix(en.d.Error, "no match")) {
		return errors.New("this one isn't waiting for somewhere to look")
	}
	if en.d.Page == "" {
		en.d.Page = en.d.URL
	}
	en.fails = 0
	en.d.URL, en.d.Resolve, en.d.Where = address, false, false
	en.d.Status, en.d.Error, en.d.Note = StatusQueued, "", ""
	e.markDirty()
	e.changed(en, true)
	e.schedule()
	return nil
}

// Pause stops a download for later; a running one is "pausing" until its run has stopped.
func (e *Engine) Pause(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	en := e.find(id)
	if en == nil {
		return ErrNotFound
	}
	freed := false
	switch {
	case en.d.Status == StatusQueued:
		en.d.Status = StatusPaused
	case en.d.Status == StatusDownloading && en.retry != nil:
		e.cancelRetry(en)
		en.d.Status, en.d.Note = StatusPaused, ""
		freed = true // it was only waiting, so the next one can start now
	case en.d.Status == StatusDownloading && en.cancel != nil:
		en.d.Status = StatusPausing
		en.cancel()
	default:
		return nil
	}
	e.markDirty()
	e.changed(en, true)
	if freed {
		e.schedule()
	}
	return nil
}

// Resume queues a paused or failed download again.
func (e *Engine) Resume(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	en := e.find(id)
	if en == nil {
		return ErrNotFound
	}
	e.resume(en)
	return nil
}

func (e *Engine) resume(en *entry) {
	switch en.d.Status {
	case StatusPaused, StatusError, StatusPausing:
		// A pausing one runs again as soon as its old run has stopped.
		en.fails = 0
		en.d.Status, en.d.Error, en.d.Note, en.d.Where = StatusQueued, "", "", false
		e.markDirty()
		e.changed(en, true)
		e.schedule()
	}
}

// Remove deletes a download and its unfinished files, plus the finished file if deleteFile. It
// returns with the download "removing"; EventRemoved follows once its run has stopped.
func (e *Engine) Remove(id string, deleteFile bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	en := e.find(id)
	if en == nil {
		return ErrNotFound
	}
	if en.d.Status == StatusRemoving {
		return nil
	}
	e.cancelRetry(en)
	e.stopChecking(en)
	en.removedFrom = en.d.Status
	en.d.Status = StatusRemoving
	e.markDirty()
	e.changed(en, true)
	go e.finishRemove(en, deleteFile)
	return nil
}

func (e *Engine) finishRemove(en *entry, deleteFile bool) {
	e.mu.Lock()
	cancel, stopped := en.cancel, en.stopped
	e.mu.Unlock()
	if cancel != nil {
		cancel()
		select {
		case <-stopped:
		case <-time.After(removeTimeout):
			e.log.Warn("download didn't stop in time; removing it anyway", "id", en.d.ID)
		}
	}

	e.mu.Lock()
	d, was := en.d, en.removedFrom
	e.mu.Unlock()
	e.discardFiles(d)
	if deleteFile && was == StatusDone && d.Path != "" {
		if err := os.Remove(d.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			e.log.Warn("couldn't delete the file", "path", d.Path, "err", err)
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.entries = slices.DeleteFunc(e.entries, func(other *entry) bool { return other == en })
	e.markDirty()
	e.emit(EventRemoved, Download{ID: d.ID})
	e.schedule()
}

// checkWritable creates dir if needed and makes sure files can be written in it.
func checkWritable(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".snag-write-test-*")
	if err != nil {
		return fmt.Errorf("can't write to %s", dir)
	}
	f.Close()
	return os.Remove(f.Name())
}

// PauseAll pauses every download that is running or waiting to.
func (e *Engine) PauseAll() {
	for _, d := range e.List() {
		if d.Status == StatusQueued || d.Status == StatusDownloading {
			_ = e.Pause(d.ID)
		}
	}
}

// ResumeAll resumes paused and retries failed downloads, oldest first so they keep their places.
func (e *Engine) ResumeAll() {
	list := e.List()
	slices.Reverse(list)
	for _, d := range list {
		if d.Status == StatusPaused || d.Status == StatusError {
			_ = e.Resume(d.ID)
		}
	}
}

// ClearDone takes the finished downloads off the list, keeping their files.
func (e *Engine) ClearDone() {
	for _, d := range e.List() {
		if d.Status == StatusDone {
			_ = e.Remove(d.ID, false)
		}
	}
}
