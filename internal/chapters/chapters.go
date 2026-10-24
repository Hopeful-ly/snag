// Package chapters puts a video's chapters on a clip, since yt-dlp would embed the whole video's.
package chapters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Roham1299/snag/internal/clip"
	"github.com/Roham1299/snag/internal/platform"
)

// Chapter is one of a video's chapters, in seconds from its start.
type Chapter struct {
	Start float64 `json:"start_time"`
	End   float64 `json:"end_time"`
	Title string  `json:"title"`
}

// Parse reads yt-dlp's chapters JSON; "NA" and "null" give none.
func Parse(text string) []Chapter {
	var chs []Chapter
	if err := json.Unmarshal([]byte(text), &chs); err != nil {
		return nil
	}
	return chs
}

// Cut keeps the chapters that show during r, retimed from its start. A last chapter without an
// end runs to the clip's end.
func Cut(chs []Chapter, r clip.Range) []Chapter {
	var out []Chapter
	for _, c := range chs {
		end := c.End
		if end <= c.Start {
			end = r.To
		}
		if end <= r.From || c.Start >= r.To {
			continue
		}
		out = append(out, Chapter{Start: max(c.Start, r.From) - r.From, End: min(end, r.To) - r.From, Title: c.Title})
	}
	return out
}

// Embed replaces the chapters in video with chs, copying the streams.
func Embed(ctx context.Context, ffmpeg, video string, chs []Chapter) error {
	if len(chs) == 0 {
		return nil
	}
	if ffmpeg == "" {
		return errors.New("need ffmpeg to add chapters")
	}
	ext := filepath.Ext(video)
	meta := video + ".snag-chapters.txt"
	tmp := video + ".snag-chapters" + ext
	defer os.Remove(meta)
	if err := os.WriteFile(meta, metadata(chs), 0o644); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, ffmpeg,
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", video, "-f", "ffmetadata", "-i", meta,
		"-map", "0", "-map_metadata", "0", "-map_chapters", "1", "-c", "copy",
		tmp)
	platform.StartGroup(cmd) // no console window on Windows
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("add chapters: %w (%s)", err, bytes.TrimSpace(out))
	}
	if err := os.Rename(tmp, video); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func metadata(chs []Chapter) []byte {
	var b strings.Builder
	b.WriteString(";FFMETADATA1\n")
	for _, c := range chs {
		fmt.Fprintf(&b, "[CHAPTER]\nTIMEBASE=1/1000\nSTART=%d\nEND=%d\ntitle=%s\n",
			millis(c.Start), millis(c.End), escape(c.Title))
	}
	return []byte(b.String())
}

func millis(seconds float64) int64 {
	return int64(seconds*1000 + 0.5)
}

// escape escapes ffmpeg metadata file syntax in titles.
var escape = strings.NewReplacer(`\`, `\\`, "=", `\=`, ";", `\;`, "#", `\#`, "\n", "\\\n").Replace
