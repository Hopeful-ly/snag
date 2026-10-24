package httpdl

import (
	"log/slog"
	"os"

	"github.com/Roham1299/snag/internal/store"
)

// resumeState is saved next to the part file so a stopped download can continue.
type resumeState struct {
	URL      string     `json:"url"`
	Size     int64      `json:"size"`
	TailFrom int64      `json:"tailFrom,omitempty"`
	Segments [][3]int64 `json:"segments"` // start, pos, end of each unfinished segment
}

// saveState is a no-op for downloads the server won't resume; those start over.
func (d *download) saveState() {
	d.mu.Lock()
	if !d.resumable || d.file == nil {
		d.mu.Unlock()
		return
	}
	state := resumeState{URL: d.url, Size: d.size, TailFrom: d.segs.tailFrom}
	for _, seg := range d.segs.list {
		if seg.left() > 0 {
			state.Segments = append(state.Segments, [3]int64{seg.start, seg.pos, seg.end})
		}
	}
	d.mu.Unlock()

	if err := store.NewFile[resumeState](d.statePath()).Save(state); err != nil {
		slog.Warn("couldn't save a download's resume state", "path", d.statePath(), "err", err)
	}
}

func (d *download) loadState() bool {
	var state resumeState
	if err := store.NewFile[resumeState](d.statePath()).Load(&state); err != nil || state.Size <= 0 {
		return false
	}
	if _, err := os.Stat(d.partPath()); err != nil {
		return false
	}

	d.url = firstNonEmpty(state.URL, d.url)
	d.size, d.resumable = state.Size, true
	d.segs = segments{tailFrom: state.TailFrom, wantFrom: -1}
	for _, s := range state.Segments {
		d.segs.list = append(d.segs.list, &segment{start: s[0], pos: s[1], end: s[2]})
	}
	d.done = d.size - d.segs.remaining()
	return true
}
