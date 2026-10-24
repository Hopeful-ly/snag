package httpdl

import (
	"cmp"
	"slices"
)

const (
	minSplit = 1 << 20 // a busy part is only split when both halves get at least this much
	// Media files are fetched in order in chunks this big so their start plays while the rest
	// arrives; the playable start trails by at most connections × this.
	inOrderChunk = 1 << 20
	// The first quickStart bytes use smaller chunks so playback can start sooner.
	quickStart = 2 << 20
	quickChunk = 256 << 10
	// A busy part this far behind a byte a player waits for is split at that byte.
	wantSplit  = 2 * inOrderChunk
	unknownEnd = 1 << 62 // the end while the size is unknown: past any real file
)

// segment is an inclusive byte range one connection fills; pos is the next byte to write.
type segment struct {
	start, pos, end int64
	busy            bool
}

func (s *segment) left() int64 {
	return s.end - s.pos + 1
}

type segments struct {
	list []*segment
	// tailFrom is where a trailing MP4 index starts, fetched early so playback can start; 0 if none.
	tailFrom int64
	// wantFrom is where a player waits to read, fetched first; -1 when none waits.
	wantFrom int64
}

// newSegments plans a fresh download; size is 0 when unknown.
func newSegments(size int64, inOrder bool) segments {
	switch {
	case size == 0:
		return segments{list: []*segment{{end: unknownEnd}}, wantFrom: -1}
	case !inOrder || size <= 2*inOrderChunk:
		return segments{list: []*segment{{end: size - 1}}, wantFrom: -1}
	}
	var list []*segment
	for at := int64(0); at < size; {
		piece := int64(inOrderChunk)
		if at < quickStart {
			piece = quickChunk
		}
		list = append(list, &segment{start: at, pos: at, end: min(at+piece, size) - 1})
		at += piece
	}
	return segments{list: list, wantFrom: -1}
}

// take returns the earliest idle part (or the one holding the MP4 index), else, if split, the
// second half of the biggest busy part, else nil.
func (s *segments) take(split bool) *segment {
	var idle []*segment
	for _, seg := range s.list {
		if !seg.busy && seg.left() > 0 {
			idle = append(idle, seg)
		}
	}
	if len(idle) > 0 {
		slices.SortFunc(idle, func(a, b *segment) int { return cmp.Compare(a.pos, b.pos) })
		pick := idle[0]
		if s.tailFrom > 0 {
			if i := slices.IndexFunc(idle, func(seg *segment) bool { return seg.end >= s.tailFrom }); i >= 0 {
				pick = idle[i]
			}
		}
		if s.wantFrom >= 0 {
			if i := slices.IndexFunc(idle, func(seg *segment) bool { return seg.end >= s.wantFrom }); i >= 0 {
				pick = idle[i]
			}
		}
		pick.busy = true
		return pick
	}
	if !split {
		return nil
	}

	var biggest *segment
	for _, seg := range s.list {
		if seg.busy && seg.left() > 2*minSplit && (biggest == nil || seg.left() > biggest.left()) {
			biggest = seg
		}
	}
	if biggest == nil {
		return nil
	}
	middle := biggest.pos + biggest.left()/2
	half := &segment{start: middle, pos: middle, end: biggest.end, busy: true}
	biggest.end = middle - 1
	s.list = append(s.list, half)
	return half
}

// want prioritizes parts from off on. A busy part far behind off is split there and the new part
// returned for a connection to start on now.
func (s *segments) want(off int64) *segment {
	s.wantFrom = off
	seg := s.holding(off)
	if seg == nil || !seg.busy || off-seg.pos < wantSplit {
		return nil
	}
	half := &segment{start: off, pos: off, end: seg.end}
	seg.end = off - 1
	s.list = append(s.list, half)
	return half
}

// have returns the length of the contiguous written run starting at off, across parts.
func (s *segments) have(off int64) int64 {
	at := off
	for {
		seg := s.holding(at)
		if seg == nil || at >= seg.pos {
			return at - off
		}
		at = seg.pos
		if seg.left() > 0 {
			return at - off
		}
	}
}

func (s *segments) holding(at int64) *segment {
	for _, seg := range s.list {
		if seg.start <= at && at <= seg.end {
			return seg
		}
	}
	return nil
}

func (s *segments) remaining() int64 {
	var n int64
	for _, seg := range s.list {
		n += max(seg.left(), 0)
	}
	return n
}
