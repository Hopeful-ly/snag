package playlist

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/naming"
)

// Choice is which entries of a list to download, and how.
type Choice struct {
	Entries   []int           `json:"entries"` // in download order
	Quality   string          `json:"quality"` // a Quality's ID
	Directory string          `json:"directory"`
	Queue     string          `json:"queue"`
	Naming    naming.Template `json:"naming"`
}

// NameKind is what a list's downloads are, which picks their default naming.
type NameKind string

// The kinds of lists' downloads.
const (
	KindSongs    NameKind = "songs"
	KindVideos   NameKind = "videos"
	KindEpisodes NameKind = "episodes"
)

// KindOf returns what p's downloads are at the given quality.
func KindOf(p Playlist, quality string) NameKind {
	for _, e := range p.Entries {
		if e.Meta.Only && e.Meta.Tags.Episode > 0 {
			return KindEpisodes
		}
	}
	if q, ok := QualityOf(quality); ok && (q.Audio != "" || q.AudioOnly) {
		return KindSongs
	}
	return KindVideos
}

// Plan returns the downloads for the chosen entries. They are marked settled so no Save dialog
// asks again for each.
func Plan(p Playlist, c Choice) ([]engine.AddRequest, error) {
	q, ok := QualityOf(c.Quality)
	if !ok {
		return nil, fmt.Errorf("unknown quality %q", c.Quality)
	}
	if len(c.Entries) == 0 {
		return nil, errors.New("nothing chosen")
	}
	if err := c.Naming.Check(); err != nil {
		return nil, err
	}
	var reqs []engine.AddRequest
	seen := map[int]bool{}
	for _, n := range c.Entries {
		if n < 0 || n >= len(p.Entries) {
			return nil, fmt.Errorf("no entry %d", n)
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		e := p.Entries[n]
		names := NamingOf(p, n, c.Naming)
		reqs = append(reqs, engine.AddRequest{
			URL:       e.URL,
			Page:      e.URL,
			Title:     e.Title,
			Label:     q.Label,
			Kind:      engine.KindStream,
			Format:    q.Format,
			Sort:      q.Sort,
			Audio:     q.Audio,
			Headers:   p.Headers,
			Cookies:   p.Cookies,
			Directory: c.Directory,
			// Without this yt-dlp names the file after its own title until the tags arrive.
			Filename:  names.Name(e.Meta.Known(e.Title)),
			Naming:    names,
			Queue:     c.Queue,
			Resolve:   p.Via != "",
			Meta:      e.Meta,
			Confirmed: true,
		})
	}
	return reqs, nil
}

// NamingOf returns how t names p's n-th entry; {n} is zero-padded so files sort in order.
func NamingOf(p Playlist, n int, t naming.Template) naming.Naming {
	width := max(2, len(strconv.Itoa(len(p.Entries))))
	return naming.Naming{Template: t, Number: fmt.Sprintf("%0*d", width, n+1), List: p.Title}
}
