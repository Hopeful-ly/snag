package engine

import (
	"errors"
	"fmt"
)

// Available is where a download's bytes are and how many can be read.
type Available struct {
	Path string // the file to read; "" while there is none to read yet
	Size int64  // the whole file's size; 0 while unknown
	N    int64  // bytes readable from the offset asked about
	Done bool   // the download is finished: Path is its file, complete
	// Live is a download whose file grows from the start and whose size isn't known till it
	// ends (streams and video sites): it can be played from the start, not jumped about in.
	Live bool
	// Separate says the picture (Path) and the sound come apart while it downloads; Sound is
	// the sound's file as it fills, "" until it begins.
	Separate bool
	Sound    string
}

// ErrNotWatchable is the answer for a download that won't get any further: failed or removed.
var ErrNotWatchable = errors.New("this download can't be watched")

// Available says how much of download id can be read from offset off. A download that waits
// to run, or hasn't opened its file yet, has nothing readable; the caller asks again later.
func (e *Engine) Available(id string, off int64) (Available, error) {
	e.mu.Lock()
	en := e.find(id)
	if en == nil {
		e.mu.Unlock()
		return Available{}, ErrNotFound
	}
	d, file := en.d, en.readable
	e.mu.Unlock()

	switch d.Status {
	case StatusDone:
		return Available{Path: d.Path, Size: d.Size, N: max(d.Size-off, 0), Done: true}, nil
	case StatusError:
		return Available{}, fmt.Errorf("%w: %s", ErrNotWatchable, d.Error)
	case StatusRemoving:
		return Available{}, ErrNotWatchable
	}
	live := d.Kind == KindStream
	if file == nil {
		return Available{Size: d.Size, Live: live}, nil
	}
	// Asked outside the engine's lock: the runner takes its own lock to answer.
	a := Available{Path: file.Path(), Size: d.Size, N: file.Have(off), Live: live}
	if tracks, ok := file.(Tracks); ok {
		a.Sound, a.Separate = tracks.Sound()
	}
	return a, nil
}

// Want asks download id for its bytes from off first, for a player waiting there. A download
// that isn't running ignores it; the player asks again once it runs.
func (e *Engine) Want(id string, off int64) {
	e.mu.Lock()
	var file Readable
	if en := e.find(id); en != nil {
		file = en.readable
	}
	e.mu.Unlock()
	if file != nil {
		file.Want(off)
	}
}
