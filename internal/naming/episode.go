package naming

import (
	"regexp"
	"strconv"
	"strings"
)

// Episode is a series season and episode taken from a title or file name.
type Episode struct {
	Series  string
	Season  int
	Episode int
}

var (
	episodeSE      = regexp.MustCompile(`(?i)\bS(?:eason)?\s*0*(\d{1,2})\s*[.\-_ ]?\s*E(?:p(?:isode)?)?\s*0*(\d{1,4})\b`)
	episodeWords   = regexp.MustCompile(`(?i)\bseason\s*0*(\d{1,2})\b[^0-9]{0,24}?\b(?:episode|ep\.?|e)\s*0*(\d{1,4})\b`)
	episodeX       = regexp.MustCompile(`(?i)(?:^|[^0-9a-z])0*(\d{1,2})x0*(\d{1,3})(?:$|[^0-9])`)
	notEpisodeWord = regexp.MustCompile(`(?i)[^a-z0-9]+`)
)

// ParseEpisode reads "Series S01E03 - Name" (and S1E3, 1x03, Season 1 Episode 3) from text.
func ParseEpisode(text string) (Episode, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Episode{}, false
	}
	season, episode, ok := numbersIn(text)
	if !ok {
		stem, _ := SplitExt(text)
		season, episode, ok = numbersIn(stem)
	}
	if !ok {
		return Episode{}, false
	}
	series := seriesOf(text)
	if series == "" {
		return Episode{}, false
	}
	return Episode{Series: series, Season: season, Episode: episode}, true
}

// SameEpisode reports whether two titles or file names are the same series episode.
func SameEpisode(a, b string) bool {
	ea, oka := ParseEpisode(a)
	eb, okb := ParseEpisode(b)
	return oka && okb && ea.Season == eb.Season && ea.Episode == eb.Episode && sameSeries(ea.Series, eb.Series)
}

func numbersIn(text string) (season, episode int, ok bool) {
	if m := episodeSE.FindStringSubmatch(text); m != nil {
		return atoi(m[1]), atoi(m[2]), true
	}
	if m := episodeWords.FindStringSubmatch(text); m != nil {
		return atoi(m[1]), atoi(m[2]), true
	}
	if m := episodeX.FindStringSubmatch(text); m != nil {
		s := atoi(m[1])
		if s > 0 && s < 60 {
			return s, atoi(m[2]), true
		}
	}
	return 0, 0, false
}

func seriesOf(text string) string {
	loc := episodeSE.FindStringIndex(text)
	if loc == nil {
		loc = episodeWords.FindStringIndex(text)
	}
	if loc == nil {
		loc = episodeX.FindStringIndex(text)
	}
	if loc == nil {
		return ""
	}
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(text[:loc[0]]), "-–—:|"))
}

func sameSeries(a, b string) bool {
	return foldSeries(a) == foldSeries(b)
}

func foldSeries(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = notEpisodeWord.ReplaceAllString(s, "")
	return s
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
