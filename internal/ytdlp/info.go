package ytdlp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Roham1299/snag/internal/platform"
)

// Format is one format a video is offered in. Unknown numbers are 0 and unknown codecs "";
// yt-dlp uses "none" for a missing video or audio stream.
type Format struct {
	FormatID       string  `json:"format_id"`
	URL            string  `json:"url"`
	Ext            string  `json:"ext"`
	VCodec         string  `json:"vcodec"`
	ACodec         string  `json:"acodec"`
	Language       string  `json:"language"`
	Height         float64 `json:"height"`
	FPS            float64 `json:"fps"`
	TBR            float64 `json:"tbr"` // total bitrate, kbit/s
	ABR            float64 `json:"abr"` // audio bitrate, kbit/s
	Filesize       float64 `json:"filesize"`
	FilesizeApprox float64 `json:"filesize_approx"`
	DynamicRange   string  `json:"dynamic_range"`
}

// Info is yt-dlp's JSON description of a page or stream. The embedded Format describes it when
// it has a single format.
type Info struct {
	Format
	Type       string   `json:"_type"`
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Thumbnail  string   `json:"thumbnail"`
	Duration   float64  `json:"duration"`
	Site       string   `json:"extractor_key"`
	WebpageURL string   `json:"webpage_url"`
	Track      string   `json:"track"`
	Artists    []string `json:"artists"`
	Album      string   `json:"album"`
	Channel    string   `json:"channel"`
	ChannelID  string   `json:"channel_id"`
	LiveStatus string   `json:"live_status"` // "is_upcoming" and "is_live" haven't finished
	Language   string   `json:"language"`
	Formats    []Format `json:"formats"`
	Entries    []Info   `json:"entries"`
	// Subtitles are uploader-provided; AutomaticCaptions are generated.
	Subtitles         map[string][]Caption `json:"subtitles"`
	AutomaticCaptions map[string][]Caption `json:"automatic_captions"`
}

// Caption is one caption file for a language (vtt, srt, json3, …).
type Caption struct {
	Ext  string `json:"ext"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

// ErrUnsupported means yt-dlp has no extractor for the address.
var ErrUnsupported = errors.New("unsupported address")

// Describe runs yt-dlp to describe a page or stream. For a playlist it returns its first video.
func Describe(ctx context.Context, ytdlp string, o InfoOptions) (Info, error) {
	o.Flat = false
	info, err := describe(ctx, ytdlp, o)
	if err != nil {
		return Info{}, err
	}
	if info.Type == "playlist" {
		for _, entry := range info.Entries {
			if entry.Title != "" || len(entry.Formats) > 0 {
				return entry, nil
			}
		}
		return Info{}, errors.New("empty playlist")
	}
	return info, nil
}

func describe(ctx context.Context, ytdlp string, o InfoOptions) (Info, error) {
	cmd := exec.CommandContext(ctx, ytdlp, InfoArgs(o)...)
	platform.StartGroup(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return Info{}, ctx.Err()
		}
		return Info{}, describeError(stderr.String(), err)
	}
	return readInfo(stdout.Bytes())
}

func readInfo(data []byte) (Info, error) {
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		return Info{}, fmt.Errorf("reading yt-dlp's description: %w", err)
	}
	return info, nil
}

func describeError(stderr string, err error) error {
	message := LastError(strings.Split(stderr, "\n"))
	if strings.Contains(message, "Unsupported URL") || strings.Contains(message, "No suitable extractor") {
		return ErrUnsupported
	}
	if message == "" {
		return fmt.Errorf("yt-dlp failed: %w", err)
	}
	return errors.New(message)
}

// LastError returns the last error among lines, or else the last line.
func LastError(lines []string) string {
	last := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if parsed := Parse(line); parsed.Kind == LineError {
			last = "ERROR: " + parsed.Text
		} else if !strings.HasPrefix(last, "ERROR: ") {
			last = line
		}
	}
	return strings.TrimPrefix(last, "ERROR: ")
}

// Choice is what a ChooseArgs run picked: one format, or video and audio formats to merge.
type Choice struct {
	Title   string
	Formats []Format // in yt-dlp's order
}

// Sound reports whether f is audio only.
func (f Format) Sound() bool { return f.VCodec == "none" }

// Size is f's stated or estimated size in bytes; 0 if unknown.
func (f Format) Size() int64 { return int64(max(f.Filesize, f.FilesizeApprox)) }

// ReadChoice reads the ChoiceFile a ChooseArgs run wrote.
func ReadChoice(path string) (Choice, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Choice{}, fmt.Errorf("yt-dlp didn't say what it chose: %w", err)
	}
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		return Choice{}, fmt.Errorf("reading what yt-dlp chose: %w", err)
	}
	if info.FormatID == "" {
		return Choice{}, errors.New("yt-dlp didn't say what it chose")
	}
	choice := Choice{Title: info.Title}
	for id := range strings.SplitSeq(info.FormatID, "+") {
		f := Format{FormatID: id}
		for _, offered := range info.Formats {
			if offered.FormatID == id {
				f = offered
				break
			}
		}
		choice.Formats = append(choice.Formats, f)
	}
	return choice, nil
}
