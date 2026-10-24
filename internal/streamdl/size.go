package streamdl

import "math"

// sizeEstimate guesses a stream's total size. yt-dlp's own guess counts in-flight pieces' bytes
// against only finished pieces, so with parallel fetches it runs high and keeps climbing; here an
// in-flight piece counts as half done, and the guess is smoothed and only moves on a >2% change.
type sizeEstimate struct {
	hint     int64
	parallel int
	smooth   float64
	shown    int64
}

// update returns the total to show and whether it is a guess.
func (s *sizeEstimate) update(downloaded, exact int64, fragment, fragments int) (int64, bool) {
	if exact > 0 {
		return exact, false
	}
	var guess float64
	switch {
	case fragments > 0 && fragment >= 3:
		inFlight := min(s.parallel, max(fragments-fragment, 0))
		guess = float64(downloaded) / (float64(fragment) + float64(inFlight)/2) * float64(fragments)
		if s.hint > 0 && float64(fragment) < float64(fragments)*0.05 {
			guess = float64(s.hint) // too few pieces in to beat the quality list's figure
		}
	case s.hint > 0:
		guess = float64(s.hint)
	}
	if guess == 0 {
		return 0, true
	}

	if s.smooth == 0 {
		s.smooth = guess
	} else {
		s.smooth = s.smooth*0.9 + guess*0.1
	}
	if s.shown == 0 || math.Abs(s.smooth-float64(s.shown))/float64(s.shown) > 0.02 {
		s.shown = int64(s.smooth)
	}
	return max(s.shown, downloaded), true
}
