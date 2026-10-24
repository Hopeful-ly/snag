package ytdlp

import (
	"regexp"
	"strconv"
	"strings"
)

// LineKind says what a line of yt-dlp's output is.
type LineKind int

// The line kinds Snag acts on; everything else is LineOther.
const (
	LineOther LineKind = iota
	LineProgress
	LineTitle
	LineDestination // Line.Text is a file yt-dlp starts writing
	LineFinal       // Line.Text is the finished file
	LineError       // Line.Text is the error, without "ERROR: "
	LineWarning     // Line.Text is the warning, without "WARNING: "
	LineAlready     // Line.Text is a file an earlier run finished
	LineCut         // ffmpeg cutting a clip: Line.Progress has Downloaded and Cut
	LineChapters    // Line.Text is chapters as JSON, "NA" for none
	LineTags        // Line.Text is the tagFields JSON for TagsOf
)

// Line is one parsed line of yt-dlp's output.
type Line struct {
	Kind     LineKind
	Text     string
	Progress Progress
}

// Progress is one progress update. Unknown numbers are 0.
type Progress struct {
	Status        string // "downloading" or "finished"
	Downloaded    int64
	Total         int64
	TotalEstimate int64
	Speed         int64 // bytes per second
	FormatID      string
	Fragment      int
	Fragments     int
	Cut           float64 // seconds of the clip written
}

var progressLine = regexp.MustCompile(`^SNAGP (\S+) (\S+) (\S+) (\S+) (\S+) (\S+) (\S+) (\S+) (\S+)`)

// Parse classifies one line of output.
func Parse(line string) Line {
	if m := progressLine.FindStringSubmatch(line); m != nil {
		return Line{Kind: LineProgress, Progress: Progress{
			Status:        m[1],
			Downloaded:    number(m[2]),
			Total:         number(m[3]),
			TotalEstimate: number(m[4]),
			Speed:         number(m[5]),
			FormatID:      m[7],
			Fragment:      int(number(m[8])),
			Fragments:     int(number(m[9])),
		}}
	}
	if m := cutLine.FindStringSubmatch(line); m != nil {
		return Line{Kind: LineCut, Progress: Progress{Downloaded: cutSize(m[1], m[2]), Cut: cutTime(m[3:])}}
	}
	if m := alreadyLine.FindStringSubmatch(line); m != nil {
		return Line{Kind: LineAlready, Text: m[1]}
	}
	for prefix, kind := range map[string]LineKind{
		"SNAGT ":                   LineTitle,
		"SNAGF ":                   LineFinal,
		"SNAGC ":                   LineChapters,
		"SNAGM ":                   LineTags,
		"[download] Destination: ": LineDestination,
		"ERROR: ":                  LineError,
		"WARNING: ":                LineWarning,
	} {
		if rest, ok := strings.CutPrefix(line, prefix); ok {
			return Line{Kind: kind, Text: strings.TrimSpace(rest)}
		}
	}
	return Line{Kind: LineOther, Text: line}
}

// number parses yt-dlp numbers, which may be "NA" or fractional.
func number(s string) int64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int64(f)
}

var alreadyLine = regexp.MustCompile(`^\[download\] (.+) has already been downloaded`)

// cutLine matches ffmpeg progress while cutting a clip: "frame=  608 fps= 87 q=31.0
// size=    1280KiB time=00:00:10.27 bitrate=...", with size N/A until known.
var cutLine = regexp.MustCompile(`size=\s*(N/A|[\d.]+)\s*([kKMG]i?B|B)?\s+time=(-?)(\d+):(\d+):([\d.]+)`)

func cutSize(n, unit string) int64 {
	size, err := strconv.ParseFloat(n, 64)
	if err != nil {
		return 0
	}
	switch strings.ToUpper(strings.TrimSuffix(strings.TrimSuffix(unit, "B"), "i")) {
	case "K":
		size *= 1 << 10
	case "M":
		size *= 1 << 20
	case "G":
		size *= 1 << 30
	}
	return int64(size)
}

func cutTime(m []string) float64 {
	if m[0] == "-" {
		return 0
	}
	h, _ := strconv.ParseFloat(m[1], 64)
	min, _ := strconv.ParseFloat(m[2], 64)
	s, _ := strconv.ParseFloat(m[3], 64)
	return h*3600 + min*60 + s
}
