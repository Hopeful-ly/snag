package streamdl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Roham1299/snag/internal/platform"
)

const finishingEvery = 500 * time.Millisecond

// watchFinishing reports a post-processing step's progress from its "<name>.temp.<ext>" output
// growing beside its input, until stop is closed.
func (s *stream) watchFinishing(stop <-chan struct{}) {
	tick := time.NewTicker(finishingEvery)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			if source, written, ok := rewriting(s.job.WorkDir); ok {
				s.report.StageProgress(min(written, source), source)
			}
		}
	}
}

// rewriting returns the size of the ".temp." file in dir and of its source, taken to be the
// largest other file that isn't an unfinished piece.
func rewriting(dir string) (source, written int64, ok bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, false
	}
	found := false
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		name := e.Name()
		switch {
		case strings.Contains(name, ".temp."):
			written, found = max(written, info.Size()), true
		case strings.HasSuffix(name, ".part") || strings.HasSuffix(name, ".ytdl") || strings.Contains(name, ".part-Frag"):
		default:
			source = max(source, info.Size())
		}
	}
	return source, written, found && source > 0
}

// isHLS reports whether address is an HLS playlist, whose pieces yt-dlp joins as MPEG-TS.
func isHLS(address string) bool {
	path, _, _ := strings.Cut(strings.ToLower(address), "?")
	return strings.HasSuffix(path, ".m3u8")
}

// isMPEGTS sniffs for MPEG-TS: 188-byte packets each starting with 0x47.
func isMPEGTS(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 189)
	if _, err := io.ReadFull(f, head); err != nil {
		return false
	}
	return head[0] == 0x47 && head[188] == 0x47
}

// fixContainer remuxes a finished MPEG-TS stream into MP4. Normally the tagging step does this
// (one rewrite instead of two); this catches files that step left alone.
func (s *stream) fixContainer(ctx context.Context) error {
	if s.final == "" || !strings.EqualFold(filepath.Ext(s.final), ".mp4") || !isMPEGTS(s.final) {
		return nil
	}
	if s.programs.FFmpeg == "" {
		return errors.New("the video needs ffmpeg to be put in an MP4, and ffmpeg is missing")
	}
	s.report.Stage("Fixing the video's container")
	stop := make(chan struct{})
	defer close(stop)
	go s.watchFinishing(stop)

	temp := strings.TrimSuffix(s.final, filepath.Ext(s.final)) + ".temp.mp4"
	cmd := exec.CommandContext(ctx, s.programs.FFmpeg, "-hide_banner", "-loglevel", "error", "-y",
		"-i", s.final, "-map", "0", "-dn", "-ignore_unknown", "-c", "copy", "-f", "mp4", "-movflags", "+faststart", temp)
	platform.StartGroup(cmd)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		_ = os.Remove(temp)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("couldn't put the video in an MP4: %w: %s", err, strings.TrimSpace(out.String()))
	}
	s.report.Said("Put the video in an MP4 (its pieces came as MPEG-TS)")
	return os.Rename(temp, s.final)
}
