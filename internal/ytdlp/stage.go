package ytdlp

import "regexp"

// stages maps yt-dlp's post-processor line prefixes ("[Merger] Merging formats into …") to
// user-facing labels; some of these steps rewrite the whole file and take a while.
var stages = map[string]string{
	"Merger":                   "Joining picture and sound",
	"FixupM3u8":                "Fixing the video's container",
	"FixupM4a":                 "Fixing the sound's container",
	"FixupTimestamp":           "Fixing the video's timing",
	"FixupDuplicateMoov":       "Fixing the video's container",
	"FixupStretched":           "Fixing the video's shape",
	"ExtractAudio":             "Converting the sound",
	"VideoConvertor":           "Converting the video",
	"VideoRemuxer":             "Putting the video in its container",
	"EmbedSubtitle":            "Adding subtitles",
	"FFmpegSubtitlesConvertor": "Converting subtitles",
	"SubtitlesConvertor":       "Converting subtitles",
	"Metadata":                 "Adding its details",
	"EmbedThumbnail":           "Adding its cover",
	"ThumbnailsConvertor":      "Adding its cover",
	"ModifyChapters":           "Cutting chapters",
	"SplitChapters":            "Cutting chapters",
	"SponsorBlock":             "Cutting chapters",
	"MoveFiles":                "Moving it into place",
}

var stageName = regexp.MustCompile(`^\[(\w+)\] `)

// StageOf returns the post-processing step a line announces, if any.
func StageOf(line string) (string, bool) {
	m := stageName.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	stage, ok := stages[m[1]]
	return stage, ok
}

// Chatter reports whether a line is one Snag asked yt-dlp to print for itself, not for the user.
func Chatter(line Line) bool {
	switch line.Kind {
	case LineTitle, LineFinal, LineChapters, LineTags, LineProgress, LineCut:
		return true
	}
	return false
}
