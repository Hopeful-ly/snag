package engine

import (
	"os"
	"regexp"
	"strings"

	"github.com/Roham1299/snag/internal/clip"
	"github.com/Roham1299/snag/internal/naming"
)

// HaveQuery is one quality or file the card is about to offer.
type HaveQuery struct {
	URL      string
	Page     string
	Title    string
	Label    string
	Filename string
	Format   string
	Audio    string
	Clip     clip.Range // a clip is neither the whole video nor another clip
}

var qualityInLabel = regexp.MustCompile(`(?i)(\d{3,4})p`)

// Have finds a download the list already has for q: the same address and quality, the same
// page and quality (stream links expire), or the same episode at that quality. A finished
// download whose file is gone does not count.
func (e *Engine) Have(q HaveQuery) (Download, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	var best *Download
	for _, en := range e.entries {
		d := &en.d
		if d.Status == StatusRemoving || !haveMatch(q, d) || !haveFile(d) {
			continue
		}
		if best == nil || haveBetter(*d, *best) {
			copy := *d
			best = &copy
		}
	}
	if best == nil {
		return Download{}, false
	}
	return *best, true
}

func haveMatch(q HaveQuery, d *Download) bool {
	if q.Clip != d.Clip {
		return false
	}
	if q.URL != "" && q.URL == d.URL && q.Format == d.Format && q.Audio == d.Audio {
		return true
	}
	if q.Page != "" && q.Page == d.Page && q.Title != "" && q.Title == d.Title &&
		q.Label == d.Label && q.Format == d.Format && q.Audio == d.Audio {
		return true
	}
	if qualityKey(q.Label) == "" || qualityKey(q.Label) != qualityKey(d.Label) {
		return false
	}
	for _, a := range haveNames(q.Title, q.Filename) {
		for _, b := range haveNames(d.Title, d.Filename) {
			if naming.SameEpisode(a, b) {
				return true
			}
		}
	}
	return false
}

func haveNames(title, filename string) []string {
	var out []string
	if title != "" {
		out = append(out, title)
	}
	if filename != "" && filename != title {
		out = append(out, filename)
	}
	return out
}

// haveFile reports whether a finished download still has its file. Unfinished ones count without one.
func haveFile(d *Download) bool {
	if d.Status != StatusDone {
		return true
	}
	if d.Path == "" {
		return false
	}
	_, err := os.Stat(d.Path)
	return err == nil
}

func qualityKey(label string) string {
	m := qualityInLabel.FindStringSubmatch(label)
	if m == nil {
		return strings.ToLower(strings.TrimSpace(label))
	}
	return strings.ToLower(m[1]) + "p"
}

func haveBetter(a, b Download) bool {
	if ra, rb := haveRank(a), haveRank(b); ra != rb {
		return ra > rb
	}
	return a.Created.After(b.Created)
}

func haveRank(d Download) int {
	switch d.Status {
	case StatusDownloading, StatusQueued, StatusPausing:
		return 4
	case StatusPaused, StatusPending:
		return 3
	case StatusDone:
		return 2
	case StatusError:
		return 1
	default:
		return 0
	}
}
