// Package ytmusic finds a song named elsewhere (e.g. Spotify) on YouTube Music by title, artists
// and length, rejecting other versions such as karaoke, covers and live takes.
package ytmusic

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/Roham1299/snag/internal/ytdlp"
)

// Song is what is known of the song to find.
type Song struct {
	Title    string
	Artists  []string
	Album    string
	Duration time.Duration // 0 if unknown
}

// Result is a search result as yt-dlp describes it.
type Result = ytdlp.Info

// Searcher returns the first n results of query: a YouTube Music search URL or a yt-dlp search prefix.
type Searcher func(ctx context.Context, query string, n int) ([]Result, error)

// Finder finds songs.
type Finder struct {
	Search Searcher
}

// New returns a Finder that searches with yt-dlp.
func New(ytdlpPath string, p ytdlp.Programs) Finder {
	return Finder{Search: func(ctx context.Context, query string, n int) ([]Result, error) {
		return ytdlp.Search(ctx, ytdlpPath, p, query, n)
	}}
}

// ErrNoMatch means no result was the song.
var ErrNoMatch = errors.New("no match")

// Results weighed per search; places the user names are searched more widely.
const (
	candidates = 3
	elsewhere  = 8
)

// Places a song can be looked for when YouTube Music hasn't got it.
const (
	PlaceYouTube    = "youtube"
	PlaceSoundCloud = "soundcloud"
)

// Find returns the address of the song on YouTube Music, else on YouTube.
func (f Finder) Find(ctx context.Context, s Song) (string, error) {
	artist := ""
	if len(s.Artists) > 0 {
		artist = s.Artists[0]
	}
	queries := []string{
		"https://music.youtube.com/search?q=" + url.QueryEscape(strings.TrimSpace(artist+" "+s.Title)) + "#songs",
		"ytsearch" + fmt.Sprint(candidates) + ":" + strings.TrimSpace(strings.Join(s.Artists, ", ")+" - "+s.Title),
	}
	var lastErr error
	for _, q := range queries {
		results, err := f.Search(ctx, q, candidates)
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err != nil {
			lastErr = err
			continue
		}
		if best, ok := Pick(s, results); ok {
			return best.WebpageURL, nil
		}
	}
	name := fmt.Sprintf("“%s”", s.Title)
	if artist != "" {
		name += " by " + artist
	}
	if lastErr != nil {
		return "", fmt.Errorf("couldn't find %s on YouTube Music: %w", name, lastErr)
	}
	return "", fmt.Errorf("couldn't find %s on YouTube Music: %w", name, ErrNoMatch)
}

// FindOn looks for the song on place. The results are returned even with ErrNoMatch, so the
// user can pick one.
func (f Finder) FindOn(ctx context.Context, s Song, place string) (string, []Result, error) {
	prefix := map[string]string{PlaceYouTube: "ytsearch", PlaceSoundCloud: "scsearch"}[place]
	if prefix == "" {
		return "", nil, fmt.Errorf("can't look on %s", place)
	}
	query := prefix + fmt.Sprint(elsewhere) + ":" + strings.TrimSpace(strings.Join(s.Artists, ", ")+" - "+s.Title)
	results, err := f.Search(ctx, query, elsewhere)
	if err != nil {
		return "", nil, err
	}
	if best, ok := Pick(s, results); ok {
		return best.WebpageURL, results, nil
	}
	return "", results, ErrNoMatch
}

// Pick returns the result most like s, if any is it.
func Pick(s Song, results []Result) (Result, bool) {
	best, bestScore := Result{}, 0.0
	for _, r := range results {
		if score, ok := Score(s, r); ok && score > bestScore {
			best, bestScore = r, score
		}
	}
	return best, bestScore > 0
}

// versions reject a result unless the song's own title or album has them too.
var versions = []string{
	"karaoke", "originally performed", "in the style of", "made famous", "tribute", "cover",
	"instrumental", "remix", "live", "acoustic", "sped up", "slowed", "nightcore", "reverb",
	"8d", "lullaby", "piano version", "music box",
}

// Title parts that aren't the song's words: "(feat. …)", "- Remastered 2011".
var (
	featuring = regexp.MustCompile(`(?i)[(\[]\s*(?:feat|ft|with)\.?\s[^)\]]*[)\]]|\s(?:feat|ft)\.?\s.*$`)
	afterDash = regexp.MustCompile(`\s+-\s+.*$`)
)

// Score rates how much r is like s, and reports whether it is s at all.
func Score(s Song, r Result) (float64, bool) {
	resultTitle := words(r.Track + " " + r.Title)
	about := resultTitle + " " + words(r.Album)

	credited := words(strings.Join(r.Artists, " ") + " " + r.Channel + " " + r.Title)
	artistOK := len(s.Artists) == 0
	for _, a := range s.Artists {
		if a := words(a); a != "" && contains(credited, a) {
			artistOK = true
		}
	}
	if !artistOK {
		return 0, false
	}

	songWords := words(s.Title + " " + s.Album)
	for _, v := range versions {
		if contains(about, v) && !contains(songWords, v) {
			return 0, false
		}
	}

	core := strings.Fields(words(afterDash.ReplaceAllString(featuring.ReplaceAllString(s.Title, ""), "")))
	if len(core) == 0 {
		core = strings.Fields(words(s.Title))
	}
	found := 0
	for _, w := range core {
		if contains(resultTitle, w) {
			found++
		}
	}
	titleScore := float64(found) / float64(max(1, len(core)))
	if titleScore < 0.6 {
		return 0, false
	}

	// Length must be within 7 s or 5 %.
	lengthScore := 0.5 // length unknown
	if s.Duration > 0 && r.Duration > 0 {
		diff := math.Abs(s.Duration.Seconds() - r.Duration)
		limit := math.Max(7, 0.05*s.Duration.Seconds())
		if diff > limit {
			return 0, false
		}
		lengthScore = 1 - diff/limit
	}

	score := 2*titleScore + lengthScore
	if s.Album != "" && words(r.Album) == words(s.Album) {
		score += 0.5
	}
	if r.Track != "" {
		score += 0.25 // a YouTube Music song, not a video
	}
	return score, true
}

// words lower-cases text and strips punctuation, padded with a space either end: " crazy in love ".
func words(text string) string {
	var b strings.Builder
	b.WriteByte(' ')
	space := true
	for _, r := range strings.ToLower(text) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			space = false
		case r == '\'' || r == '’': // "don't" is one word
		case !space:
			b.WriteByte(' ')
			space = true
		}
	}
	if !space {
		b.WriteByte(' ')
	}
	if b.Len() == 1 {
		return ""
	}
	return b.String()
}

// contains reports whether text, from words, has phrase as whole words.
func contains(text, phrase string) bool {
	phrase = strings.TrimSpace(words(phrase))
	return phrase != "" && strings.Contains(text, " "+phrase+" ")
}
