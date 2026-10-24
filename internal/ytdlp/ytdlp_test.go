package ytdlp

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Roham1299/snag/internal/clip"
	"github.com/Roham1299/snag/internal/meta"
)

func TestParseProgress(t *testing.T) {
	got := Parse("SNAGP downloading 1048576 NA 52428800.5 250000.0 12 hls-1080p 3 200")
	want := Line{Kind: LineProgress, Progress: Progress{
		Status: "downloading", Downloaded: 1048576, TotalEstimate: 52428800, Speed: 250000,
		FormatID: "hls-1080p", Fragment: 3, Fragments: 200,
	}}
	if got != want {
		t.Errorf("Parse = %+v\nwant  %+v", got, want)
	}
}

func TestParseCut(t *testing.T) {
	tests := []struct {
		line string
		want Progress
	}{
		{"frame=  608 fps= 87 q=31.0 size=    1280KiB time=00:00:10.27 bitrate=1020.9kbits/s speed=1.47x",
			Progress{Downloaded: 1280 << 10, Cut: 10.27}},
		{"size=     512kB time=01:02:03.50 bitrate= 438.4kbits/s", Progress{Downloaded: 512 << 10, Cut: 3723.5}},
		{"frame=    0 fps=0.0 q=0.0 size=N/A time=-00:00:00.05 bitrate=N/A", Progress{}},
		{"frame=  702 fps= 96 q=-1.0 Lsize=    1761KiB time=00:00:11.70 bitrate=1232.9kbits/s",
			Progress{Downloaded: 1761 << 10, Cut: 11.7}},
	}
	for _, tt := range tests {
		got := Parse(tt.line)
		if got.Kind != LineCut || got.Progress != tt.want {
			t.Errorf("Parse(%q) = %v %+v, want a cut with %+v", tt.line, got.Kind, got.Progress, tt.want)
		}
	}
}

func TestParseOtherLines(t *testing.T) {
	tests := []struct {
		line string
		kind LineKind
		text string
	}{
		{"SNAGT A Film: Part 2", LineTitle, "A Film: Part 2"},
		{"SNAGF /work/A Film.mp4", LineFinal, "/work/A Film.mp4"},
		{`SNAGC [{"start_time": 0, "title": "Intro", "end_time": 76}]`, LineChapters, `[{"start_time": 0, "title": "Intro", "end_time": 76}]`},
		{"[download] Destination: /work/A Film.mp4", LineDestination, "/work/A Film.mp4"},
		{"[download] /work/A Film.f140.m4a has already been downloaded", LineAlready, "/work/A Film.f140.m4a"},
		{"ERROR: [generic] Unable to download webpage: HTTP Error 403: Forbidden", LineError,
			"[generic] Unable to download webpage: HTTP Error 403: Forbidden"},
		{"WARNING: falling back", LineWarning, "falling back"},
		{"[hlsnative] Downloading m3u8 manifest", LineOther, "[hlsnative] Downloading m3u8 manifest"},
	}
	for _, tt := range tests {
		if got := Parse(tt.line); got.Kind != tt.kind || got.Text != tt.text {
			t.Errorf("Parse(%q) = %v %q, want %v %q", tt.line, got.Kind, got.Text, tt.kind, tt.text)
		}
	}
}

func TestDownloadArgs(t *testing.T) {
	args := DownloadArgs(DownloadOptions{
		URL: "https://example.com/watch?v=1", Dir: "/work", Name: "100% Real",
		Format: "bv*+ba/b", Connections: 4, CookieFile: "/work/cookies.txt",
		Headers:  map[string]string{"Referer": "https://example.com/", "Cookie": "x=1", "Range": "bytes=0-"},
		Programs: Programs{FFmpeg: "/tools/ffmpeg"},
	})
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--ignore-config", "-P /work", "-o 100%% Real.%(ext)s", "-f bv*+ba/b", "--concurrent-fragments 4",
		"--merge-output-format mp4/mkv", "--add-header Referer:https://example.com/",
		"--cookies /work/cookies.txt", "--ffmpeg-location /tools/ffmpeg",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("arguments lack %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "Cookie:") || strings.Contains(joined, "Range:") {
		t.Errorf("passed on a header it must not: %s", joined)
	}
	if args[len(args)-2] != "--" || args[len(args)-1] != "https://example.com/watch?v=1" {
		t.Errorf("the address must come last, after --: %v", args[len(args)-2:])
	}
}

func TestPartArgs(t *testing.T) {
	args := DownloadArgs(DownloadOptions{
		URL: "https://example.com/watch?v=1", Dir: "/work", Name: "100% Real", Format: "dash-audio=128000",
		InfoFile: "/work/" + ChoiceFile, Part: true, Headers: map[string]string{"Referer": "https://example.com/"},
	})
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-o 100%% Real.fdash-audio=128000.%(ext)s", "-f dash-audio=128000", "--fixup never",
		"--load-info-json /work/" + ChoiceFile, "--add-header Referer:https://example.com/",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("arguments lack %q:\n%s", want, joined)
		}
	}
	for _, unwanted := range []string{"--merge-output-format", "--embed-metadata", "https://example.com/watch"} {
		if strings.Contains(joined, unwanted) {
			t.Errorf("a part run has %q:\n%s", unwanted, joined)
		}
	}

	if limited := strings.Join(DownloadArgs(DownloadOptions{URL: "u", Dir: "/w", LimitRate: 250000}), " "); !strings.Contains(limited, "--limit-rate 250000") {
		t.Errorf("a speed limit isn't passed on: %s", limited)
	}

	choose := strings.Join(ChooseArgs(DownloadOptions{URL: "u", Dir: "/work", Sort: "res:1080"}), " ")
	for _, want := range []string{"--skip-download", "--write-info-json", "-o infojson:snag-choice", "-f bv*+ba/b",
		"-S res:1080", "-- u"} {
		if !strings.Contains(choose, want) {
			t.Errorf("choosing arguments lack %q:\n%s", want, choose)
		}
	}
}

func parseActions(args []string) []string {
	var actions []string
	for i, a := range args {
		if a == "--parse-metadata" {
			actions = append(actions, args[i+1])
		}
	}
	return actions
}

func TestLabelArgs(t *testing.T) {
	plain := strings.Join(DownloadArgs(DownloadOptions{URL: "u", Dir: "/w"}), " ")
	if strings.Contains(plain, "--parse-metadata") || strings.Contains(plain, "--embed-thumbnail") {
		t.Errorf("a video without a label changes the source's: %s", plain)
	}

	episode := meta.Label{Only: true, Tags: meta.Tags{Title: "Pilot", Show: "Breaking Bad: 100%", Season: 1,
		Episode: 2, Date: "2008-01-20", Page: "https://site.example/watch/tv/1396/1/2"}}
	actions := parseActions(DownloadArgs(DownloadOptions{URL: "u", Dir: "/w", Meta: episode}))
	for _, want := range []string{
		"%(snag|)sPilot:%(meta_title)s",
		`%(snag|)sBreaking Bad\: 100%%:%(meta_show)s`,
		"%(snag|)s1:%(meta_season_number)s",
		"%(snag|)s2:%(meta_episode_sort)s",
		"%(snag|)s20080120:%(meta_date)s",
		`%(snag|)shttps\://site.example/watch/tv/1396/1/2:%(meta_comment)s`,
		":(?P<meta_description>)",
		":(?P<meta_album>)",
	} {
		if !slices.Contains(actions, want) {
			t.Errorf("episode actions lack %q: %q", want, actions)
		}
	}
	if slices.Contains(actions, "%(track,title|)s:(?P<meta_title>.+)") {
		t.Errorf("an episode keeps the stream's title: %q", actions)
	}

	inList := parseActions(DownloadArgs(DownloadOptions{URL: "u", Dir: "/w",
		Meta: meta.Label{Defaults: meta.Tags{Album: "Mix", Track: 7}}}))
	if want := []string{
		"%(snag|)sMix:%(meta_album)s", "%(album|)s:(?P<meta_album>.+)",
		"%(snag|)s7:%(meta_track)s", "%(track_number|)s:(?P<meta_track>.+)",
	}; !slices.Equal(inList, want) {
		t.Errorf("list actions %q, want %q", inList, want)
	}

	part := strings.Join(DownloadArgs(DownloadOptions{URL: "u", Dir: "/w", Meta: episode, Part: true}), " ")
	if strings.Contains(part, "--parse-metadata") {
		t.Errorf("a part run is labelled: %s", part)
	}
}

func TestSongArgs(t *testing.T) {
	args := DownloadArgs(DownloadOptions{URL: "u", Dir: "/w", ExtractAudio: "mp3",
		Meta: meta.Label{Defaults: meta.Tags{Album: "Mix", Track: 7}}})
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--embed-thumbnail --convert-thumbnails jpg") {
		t.Errorf("a song has no cover: %s", joined)
	}
	actions := parseActions(args)
	album := slices.Index(actions, "%(album|)s:(?P<meta_album>.+)")
	artist := slices.IndexFunc(actions, func(a string) bool { return strings.Contains(a, "meta_album_artist") })
	if album < 0 || artist < album {
		t.Errorf("song actions out of order: %q", actions)
	}
	video := strings.Join(DownloadArgs(DownloadOptions{URL: "u", Dir: "/w",
		Meta: meta.Label{Defaults: meta.Tags{Album: "Mix"}}}), " ")
	if strings.Contains(video, "--embed-thumbnail") {
		t.Errorf("a video is treated as a song: %s", video)
	}

	named := DownloadArgs(DownloadOptions{URL: "u", Dir: "/w", ExtractAudio: "mp3", Meta: meta.Label{
		Tags: meta.Tags{Title: "Get Lucky", Artist: "Daft Punk", Cover: "https://i.example/c.jpg"}}})
	if strings.Contains(strings.Join(named, " "), "--embed-thumbnail") {
		t.Errorf("a song with its own cover gets the video's too: %q", named)
	}
	actions = parseActions(named)
	if n := len(actions); n < 2 || actions[n-2] != "%(snag|)sGet Lucky:%(meta_title)s" ||
		actions[n-1] != "%(snag|)sDaft Punk:%(meta_artist)s" {
		t.Errorf("a named song's tags don't come last: %q", actions)
	}
}

func TestReadChoice(t *testing.T) {
	path := filepath.Join(t.TempDir(), ChoiceFile)
	write := func(s string) {
		if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"title": "A Film", "format_id": "137+140", "formats": [
		{"format_id": "140", "vcodec": "none", "acodec": "mp4a", "filesize": 3000},
		{"format_id": "137", "vcodec": "avc1", "acodec": "none", "filesize_approx": 90000}]}`)
	c, err := ReadChoice(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Title != "A Film" || len(c.Formats) != 2 || c.Formats[0].FormatID != "137" || c.Formats[0].Sound() ||
		c.Formats[0].Size() != 90000 || !c.Formats[1].Sound() || c.Formats[1].Size() != 3000 {
		t.Errorf("choice = %+v", c)
	}

	write(`{"title": "One", "format_id": "hls-720"}`)
	if c, err := ReadChoice(path); err != nil || len(c.Formats) != 1 || c.Formats[0].FormatID != "hls-720" {
		t.Errorf("one format: %+v, %v", c, err)
	}
	write(`{"title": "None"}`)
	if _, err := ReadChoice(path); err == nil {
		t.Error("a description without a choice was read as one")
	}
}

func TestCaptionArgs(t *testing.T) {
	args := strings.Join(CaptionArgs(DownloadOptions{
		URL: "https://example.com/v", Dir: "/work", CookieFile: "/work/cookies.txt",
	}, "en-orig", true), " ")
	for _, want := range []string{
		"--skip-download", "--write-subs", "--write-auto-subs", "--convert-subs srt",
		"--sub-langs en-orig", "-P /work", "-o " + CaptionsFile, "--cookies /work/cookies.txt",
		"-- https://example.com/v",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("arguments lack %q:\n%s", want, args)
		}
	}
	uploader := strings.Join(CaptionArgs(DownloadOptions{URL: "u", Dir: "d"}, "en", false), " ")
	if strings.Contains(uploader, "--write-auto-subs") {
		t.Errorf("an uploader's captions also ask for automatic ones:\n%s", uploader)
	}
}

func TestDownloadArgsLeaveCaptionsOut(t *testing.T) {
	for _, args := range [][]string{
		DownloadArgs(DownloadOptions{URL: "u", Dir: "d", Format: "bv*+ba"}),
		DownloadArgs(DownloadOptions{URL: "u", Dir: "d", Format: "137", Part: true}),
		DownloadArgs(DownloadOptions{URL: "u", Dir: "d", ExtractAudio: "mp3"}),
		DownloadArgs(DownloadOptions{URL: "u", Dir: "d", Clip: clip.Range{From: 1, To: 2}}),
	} {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "subs") {
			t.Errorf("the video's run fetches captions:\n%s", joined)
		}
	}
}

func TestDownloadArgsForAClip(t *testing.T) {
	args := DownloadArgs(DownloadOptions{
		URL: "https://example.com/watch?v=1", Dir: "/work", Format: "bv*+ba/b",
		Clip: clip.Range{From: 62.5, To: 74},
	})
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--download-sections *62.5-74", "--force-keyframes-at-cuts", "--merge-output-format mp4 ",
		"--downloader-args ffmpeg_o:-preset veryfast -crf 20",
		"--no-embed-chapters", "--print before_dl:SNAGC %(chapters)j",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("arguments lack %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "--write-subs") || slices.Contains(args, "--embed-subs") {
		t.Errorf("a clip's run fetches captions:\n%s", joined)
	}

	audio := strings.Join(DownloadArgs(DownloadOptions{URL: "https://example.com/a", Dir: "/w", ExtractAudio: "mp3", Clip: clip.Range{From: 1, To: 2}}), " ")
	if !strings.Contains(audio, "--download-sections *1-2") || strings.Contains(audio, "--downloader-args") {
		t.Errorf("an audio clip's arguments are wrong:\n%s", audio)
	}
}

func TestDownloadArgsForAudio(t *testing.T) {
	args := DownloadArgs(DownloadOptions{URL: "u", Dir: "d", Format: "ba/b", ExtractAudio: "mp3"})
	if !slices.Contains(args, "-x") || slices.Contains(args, "--merge-output-format") {
		t.Errorf("audio arguments: %v", args)
	}
	if !slices.Contains(args, "%(title).180B.%(ext)s") {
		t.Errorf("without a name the title should name the file: %v", args)
	}
}

func TestLastError(t *testing.T) {
	lines := []string{"[download] 10%", "ERROR: first", "some trailing line", "ERROR: HTTP Error 403: Forbidden", ""}
	if got := LastError(lines); got != "HTTP Error 403: Forbidden" {
		t.Errorf("LastError = %q", got)
	}
	if got := LastError([]string{"Killed"}); got != "Killed" {
		t.Errorf("without errors: %q", got)
	}
}

func TestTagsOf(t *testing.T) {
	if !strings.Contains(tagFields, "meta_artist,artist,uploader") || strings.Contains(tagFields, "description") {
		t.Errorf("printed %s", tagFields)
	}
	// meta_album "" clears the tag rather than falling back to "album".
	tags, err := TagsOf(`{"meta_title": "Get Lucky", "title": "Daft Punk - Get Lucky (Official Video)", "meta_album": "",
		"album": "Singles", "uploader": "Daft Punk", "track_number": 8, "meta_date": "20130520", "disc_number": "1/2"}`)
	if err != nil {
		t.Fatal(err)
	}
	want := meta.Tags{Title: "Get Lucky", Artist: "Daft Punk", Track: 8, Disc: 1, Date: "2013-05-20"}
	if tags != want {
		t.Errorf("tags %+v, want %+v", tags, want)
	}
	if _, err := TagsOf("NA"); err == nil {
		t.Error("read tags from nothing")
	}
	if l := Parse("SNAGM {\"title\": \"x\"}"); l.Kind != LineTags || l.Text != `{"title": "x"}` {
		t.Errorf("line %+v", l)
	}
}

func TestStageOf(t *testing.T) {
	for line, want := range map[string]string{
		`[Merger] Merging formats into "a.mp4"`:                  "Joining picture and sound",
		`[FixupM3u8] Fixing MPEG-TS in MP4 container of "a.mp4"`: "Fixing the video's container",
		`[EmbedSubtitle] Embedding subtitles in "a.mp4"`:         "Adding subtitles",
		`[Metadata] Adding metadata to "a.mp4"`:                  "Adding its details",
		`[ExtractAudio] Destination: a.mp3`:                      "Converting the sound",
	} {
		if got, ok := StageOf(line); !ok || got != want {
			t.Errorf("StageOf(%q) = %q, %v", line, got, ok)
		}
	}
	for _, line := range []string{`[download] Destination: a.mp4`, `[youtube] abc: Downloading webpage`, "nothing"} {
		if got, ok := StageOf(line); ok {
			t.Errorf("StageOf(%q) = %q", line, got)
		}
	}
}
