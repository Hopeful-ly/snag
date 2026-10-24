// Package clip describes a part of a video to download.
package clip

import "math"

// Range is the part of a video from From to To, in seconds; the zero Range is the whole video.
type Range struct {
	From float64 `json:"from"`
	To   float64 `json:"to"`
}

// IsSet reports whether r is a part rather than the whole video.
func (r Range) IsSet() bool {
	return r != Range{}
}

// Valid reports whether r is zero or ends after it starts.
func (r Range) Valid() bool {
	if !r.IsSet() {
		return true
	}
	finite := !math.IsInf(r.From, 0) && !math.IsNaN(r.From) && !math.IsInf(r.To, 0) && !math.IsNaN(r.To)
	return finite && r.From >= 0 && r.To > r.From
}

// Length is how long the part is, in seconds; 0 for the whole video.
func (r Range) Length() float64 {
	return r.To - r.From
}
