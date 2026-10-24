// Package ytdlp builds yt-dlp command lines and parses its progress output and JSON info.
package ytdlp

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Roham1299/snag/internal/clip"
	"github.com/Roham1299/snag/internal/meta"
)

// progressTemplate makes yt-dlp print one line Snag can parse for every progress update.
const progressTemplate = "download:SNAGP %(progress.status)s %(progress.downloaded_bytes)s " +
	"%(progress.total_bytes)s %(progress.total_bytes_estimate)s %(progress.speed)s " +
	"%(progress.eta)s %(info.format_id)s %(progress.fragment_index)s %(progress.fragment_count)s"

// Programs are paths to the helper programs yt-dlp uses; "" lets it search PATH.
type Programs struct {
	FFmpeg string
	Node   string
}

// DownloadOptions describe one download.
type DownloadOptions struct {
	URL  string
	Dir  string // also holds partial files
	Name string // without extension; "" for the video's title

	Format       string // "" for the best
	Sort         string
	ExtractAudio string // such as "mp3"; "" to keep the video
	// FixInTags skips the HLS fixup pass, leaving the remux to MP4 to the metadata step and
	// saving a full rewrite of the file.
	FixInTags bool

	Clip clip.Range
	Meta meta.Label

	// InfoFile is the info JSON from a ChooseArgs run, loaded instead of re-fetching the page.
	InfoFile string
	// Part downloads one format of a merged video, unprocessed, as name.f<format>.<ext>, the name
	// yt-dlp looks for when it later merges the parts.
	Part bool

	Connections int
	LimitRate   int64 // bytes per second; 0 for none
	Headers     map[string]string
	CookieFile  string
	Programs    Programs
}

// ChoiceFile is the info JSON file a ChooseArgs run writes.
const ChoiceFile = "snag-choice.info.json"

// ChooseArgs returns arguments that resolve formats without downloading, writing ChoiceFile
// to o.Dir and printing the title.
func ChooseArgs(o DownloadOptions) []string {
	args := common(o.Programs)
	args = append(args,
		"--newline", "--no-playlist", "--skip-download", "--write-info-json", "--no-write-playlist-metafiles",
		"--print", "video:SNAGT %(title)s", "--no-simulate", "--no-quiet",
		"-P", o.Dir, "-o", "infojson:"+strings.TrimSuffix(ChoiceFile, ".info.json"),
		"-f", cmp.Or(o.Format, defaultFormat),
	)
	if o.Sort != "" {
		args = append(args, "-S", o.Sort)
	}
	return append(args, source(o)...)
}

// DownloadArgs returns yt-dlp's arguments for a download.
func DownloadArgs(o DownloadOptions) []string {
	name := "%(title).180B"
	if o.Name != "" {
		name = escape(o.Name)
	}
	if o.Part {
		// Literal format id, not a template field, which yt-dlp would sanitize.
		name += ".f" + escape(o.Format)
	}
	args := common(o.Programs)
	args = append(args,
		"--newline", "--no-playlist", "--no-mtime",
		"--concurrent-fragments", strconv.Itoa(max(1, o.Connections)),
		"--progress", "--progress-template", progressTemplate,
		"--print", "before_dl:SNAGT %(title)s",
		"--print", "after_move:SNAGF %(filepath)s",
		"--no-simulate", "--no-quiet",
		"-P", o.Dir, "-o", name+".%(ext)s",
	)
	if o.LimitRate > 0 {
		args = append(args, "--limit-rate", strconv.FormatInt(o.LimitRate, 10))
	}
	if o.Part {
		// The merging run fixes and tags the whole.
		args = append(args, "--fixup", "never")
	} else {
		if o.FixInTags {
			args = append(args, "--fixup", "never")
		}
		args = append(args, "--embed-metadata", "--print", "after_move:SNAGM "+tagFields)
		args = append(args, labelArgs(o.Meta, o.ExtractAudio != "")...)
	}
	switch {
	case o.Format != "":
		args = append(args, "-f", o.Format)
	case o.ExtractAudio == "":
		args = append(args, "-f", defaultFormat)
	}
	if o.Sort != "" {
		args = append(args, "-S", o.Sort)
	}
	switch {
	case o.ExtractAudio != "":
		args = append(args, "-x", "--audio-format", o.ExtractAudio)
	case o.Clip.IsSet():
		args = append(args, "--merge-output-format", "mp4")
	case !o.Part && Joined(o.Format):
		args = append(args, "--merge-output-format", "mp4/mkv")
	}
	args = append(args, clipArgs(o)...)
	return append(args, source(o)...)
}

// CaptionsFile is the base name CaptionArgs writes to, as CaptionsFile.<lang>.<ext>.
const CaptionsFile = "snag-captions"

// CaptionArgs returns arguments that fetch only captions in lang into o.Dir; auto allows
// automatic captions. Captions are a separate run so a site refusing them (as YouTube often
// does) doesn't fail the video.
func CaptionArgs(o DownloadOptions, lang string, auto bool) []string {
	args := common(o.Programs)
	args = append(args,
		"--no-playlist", "--skip-download", "--write-subs", "--convert-subs", "srt",
		"--sub-langs", lang, "--sub-format", "srt/vtt/best",
		"-P", o.Dir, "-o", CaptionsFile,
	)
	if auto {
		args = append(args, "--write-auto-subs")
	}
	return append(args, source(o)...)
}

// clipArgs cut the clip frame-exactly, which forces a re-encode, so it uses a fast H.264 preset.
func clipArgs(o DownloadOptions) []string {
	if !o.Clip.IsSet() {
		return nil
	}
	sections := "*" + seconds(o.Clip.From) + "-" + seconds(o.Clip.To)
	args := []string{"--download-sections", sections, "--force-keyframes-at-cuts",
		// yt-dlp would embed the full video's chapter times; print them so Snag can shift
		// them to the clip afterwards.
		"--no-embed-chapters", "--print", "before_dl:SNAGC %(chapters)j"}
	if o.ExtractAudio == "" {
		args = append(args, "--downloader-args", "ffmpeg_o:-preset veryfast -crf 20")
	}
	return args
}

func seconds(s float64) string {
	return strconv.FormatFloat(s, 'f', -1, 64)
}

// Joined reports whether a format selector may pick separate video and audio streams to merge.
func Joined(format string) bool {
	return format == "" || strings.Contains(format, "+")
}

// source returns the request headers and cookies plus either the info file or the URL.
func source(o DownloadOptions) []string {
	args := headerArgs(o.Headers, "cookie", "range", "host", "accept-encoding")
	if o.CookieFile != "" {
		args = append(args, "--cookies", o.CookieFile)
	}
	if o.InfoFile != "" {
		return append(args, "--load-info-json", o.InfoFile)
	}
	return append(args, "--", o.URL)
}

// labelArgs tag the file per meta.Label. Order matters, as later --parse-metadata actions win:
// defaults, then the source's own fields, then songArgs, then Snag's tags.
func labelArgs(l meta.Label, song bool) []string {
	var args []string
	parse := func(action string) { args = append(args, "--parse-metadata", action) }
	if l.Only {
		for _, tag := range tagsOf(l.Tags) {
			parse(setTag(tag.key, tag.value))
		}
		return args
	}
	for _, tag := range tagsOf(l.Defaults) {
		if tag.value != "" {
			parse(setTag(tag.key, tag.value))
			parse("%(" + tag.fields + "|)s:(?P<meta_" + tag.key + ">.+)")
		}
	}
	if song {
		if l.Tags.Cover == "" { // else Snag embeds its own cover afterwards
			args = append(args, songCover...)
		}
		args = append(args, songArgs...)
	}
	for _, tag := range tagsOf(l.Tags) {
		if tag.value != "" {
			parse(setTag(tag.key, tag.value))
		}
	}
	return args
}

// tag is an ffmpeg tag key, its value, and the yt-dlp fields it otherwise comes from.
type tag struct{ key, value, fields string }

func tagsOf(t meta.Tags) []tag {
	number := func(n int) string {
		if n <= 0 {
			return ""
		}
		return strconv.Itoa(n)
	}
	return []tag{
		{"title", t.Title, "track,title"},
		{"artist", t.Artist, "artist,uploader"},
		{"album", t.Album, "album"},
		{"album_artist", t.AlbumArtist, "album_artist"},
		{"track", number(t.Track), "track_number"},
		{"disc", number(t.Disc), "disc_number"},
		{"date", strings.ReplaceAll(t.Date, "-", ""), "upload_date"},
		{"comment", t.Page, "webpage_url"},
		{"purl", t.Page, "webpage_url"},
		{"description", "", "description"},
		{"synopsis", "", "description"},
		{"show", t.Show, "series"},
		{"season_number", number(t.Season), "season_number"},
		{"episode_sort", number(t.Episode), "episode_number"},
	}
}

// songCover embeds the thumbnail cropped square, as album art.
var songCover = []string{
	"--embed-thumbnail", "--convert-thumbnails", "jpg",
	"--postprocessor-args", `ThumbnailsConvertor+ffmpeg_o:-vf crop='min(iw\,ih)':'min(iw\,ih)'`,
}

// songArgs tag audio as music players expect: release year as date, no video description, no
// "Music" genre, an album artist, and for Music-category videos with no track field titled
// "Artist - Song (Official Video)" the artist and title split from that.
var songArgs = []string{
	"--parse-metadata", `%(categories.0|)s|%(track|)s|%(title)s:^Music\|\|(?P<meta_artist>.+?) - (?P<meta_title>.+)$`,
	"--replace-in-metadata", "meta_title",
	`(?i)\s*[(\[][^)\]]*\b(?:official|lyrics?|audio|video|visuali[sz]er|hd|hq|4k)\b[^)\]]*[)\]]`, "",
	"--parse-metadata", `%(album&yes|)s|%(album_artist,artists.0|)s:^yes\|(?P<meta_album_artist>.+)$`,
	"--parse-metadata", `%(release_year|)s:(?P<meta_date>.+)`,
	"--parse-metadata", ":(?P<meta_description>)",
	"--parse-metadata", ":(?P<meta_synopsis>)",
	"--parse-metadata", ":(?P<meta_genre>)",
	"--parse-metadata", `%(genre,genres.0|)s:^(?!Music$)(?P<meta_genre>.+)$`,
}

// setTag is the --parse-metadata action setting tag key to value, or clearing it for "".
// yt-dlp reads a lone word as a field name, so the always-empty "%(snag|)s" prefix forces it to
// be read as a template.
func setTag(key, value string) string {
	if value == "" {
		return ":(?P<meta_" + key + ">)"
	}
	value = strings.ReplaceAll(escape(value), ":", `\:`)
	return "%(snag|)s" + value + ":%(meta_" + key + ")s"
}

func escape(name string) string {
	return strings.ReplaceAll(name, "%", "%%")
}

const defaultFormat = "bv*+ba/b"

// InfoOptions describe a request for what a page or stream offers.
type InfoOptions struct {
	URL string
	// Stream forces the generic extractor, treating the URL as a stream manifest.
	Stream bool
	// Flat lists the playlist the page is in, or is, rather than the video.
	Flat       bool
	Max        int // flat entries to list; 0 for MaxEntries
	Headers    map[string]string
	CookieFile string
	Programs   Programs
}

// MaxEntries caps the entries listed for a flat playlist.
const MaxEntries = 1000

// InfoArgs returns yt-dlp's arguments for describing a page or stream as JSON.
func InfoArgs(o InfoOptions) []string {
	extractors := "default,-generic"
	if o.Stream {
		extractors = "generic"
	}
	args := common(o.Programs)
	args = append(args, "-J", "--socket-timeout", "15", "--use-extractors", extractors)
	if o.Flat {
		most := MaxEntries
		if o.Max > 0 {
			most = min(o.Max, MaxEntries)
		}
		args = append(args, "--yes-playlist", "--flat-playlist", "--playlist-end", strconv.Itoa(most))
	} else {
		args = append(args, "--no-playlist")
	}
	for name, value := range o.Headers {
		if lower := strings.ToLower(name); lower == "referer" || lower == "user-agent" {
			args = append(args, "--add-header", name+":"+value)
		}
	}
	if o.CookieFile != "" {
		args = append(args, "--cookies", o.CookieFile)
	}
	return append(args, "--", o.URL)
}

// common ignores the user's yt-dlp config, which could change the output Snag parses.
func common(p Programs) []string {
	args := []string{"--ignore-config", "--color", "never", "--encoding", "utf-8"}
	if p.FFmpeg != "" {
		args = append(args, "--ffmpeg-location", p.FFmpeg)
	}
	if p.Node != "" {
		args = append(args, "--js-runtimes", "node:"+p.Node)
	}
	return args
}

func headerArgs(headers map[string]string, skip ...string) []string {
	var args []string
outer:
	for name, value := range headers {
		for _, s := range skip {
			if strings.EqualFold(name, s) {
				continue outer
			}
		}
		args = append(args, "--add-header", name+":"+value)
	}
	return args
}

// tagFields is a --print template dumping, as JSON, each meta_* tag and its fallback fields.
var tagFields = func() string {
	var names []string
	for _, t := range tagsOf(meta.Tags{}) {
		if !named[t.key] {
			continue
		}
		names = append(names, "meta_"+t.key)
		names = append(names, strings.Split(t.fields, ",")...)
	}
	return "%(.{" + strings.Join(slices.Compact(names), ",") + "})j"
}()

// named are the tags naming templates use.
var named = map[string]bool{
	"title": true, "artist": true, "album": true, "album_artist": true, "track": true, "disc": true,
	"date": true, "show": true, "season_number": true, "episode_sort": true,
}

// TagsOf parses the JSON tagFields printed.
func TagsOf(printed string) (meta.Tags, error) {
	var fields map[string]any
	if err := json.Unmarshal([]byte(printed), &fields); err != nil {
		return meta.Tags{}, fmt.Errorf("reading the file's tags: %w", err)
	}
	value := func(t tag) string {
		if v, ok := fields["meta_"+t.key].(string); ok {
			return strings.TrimSpace(v) // "" means deliberately cleared
		}
		for _, name := range strings.Split(t.fields, ",") {
			switch v := fields[name].(type) {
			case string:
				if v = strings.TrimSpace(v); v != "" {
					return v
				}
			case float64:
				return strconv.FormatFloat(v, 'f', -1, 64)
			}
		}
		return ""
	}
	number := func(s string) int {
		s, _, _ = strings.Cut(s, "/") // "3/12"
		n, _ := strconv.Atoi(strings.TrimSpace(s))
		return n
	}
	var tags meta.Tags
	for _, t := range tagsOf(meta.Tags{}) {
		if !named[t.key] {
			continue
		}
		v := value(t)
		switch t.key {
		case "title":
			tags.Title = v
		case "artist":
			tags.Artist = v
		case "album":
			tags.Album = v
		case "album_artist":
			tags.AlbumArtist = v
		case "track":
			tags.Track = number(v)
		case "disc":
			tags.Disc = number(v)
		case "date":
			tags.Date = v
			if len(v) == 8 && number(v) > 0 { // yt-dlp's 20130520
				tags.Date = v[:4] + "-" + v[4:6] + "-" + v[6:]
			}
		case "show":
			tags.Show = v
		case "season_number":
			tags.Season = number(v)
		case "episode_sort":
			tags.Episode = number(v)
		}
	}
	return tags, nil
}
