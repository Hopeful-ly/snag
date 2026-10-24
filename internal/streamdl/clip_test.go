package streamdl

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/clip"
)

func TestCutsAClip(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("needs ffmpeg")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("needs ffprobe")
	}
	dir := t.TempDir()
	source := []string{"-loglevel", "error", "-f", "lavfi", "-i", "testsrc=duration=10:size=160x120:rate=25",
		"-f", "lavfi", "-i", "sine=duration=10", "-c:v", "libx264", "-g", "50", "-c:a", "aac"}
	for _, out := range [][]string{
		{"-f", "hls", "-hls_time", "2", "-hls_playlist_type", "vod", filepath.Join(dir, "show.m3u8")},
		{"-movflags", "+faststart", filepath.Join(dir, "film.mp4")},
	} {
		if out, err := exec.Command(ffmpeg, append(source, out...)...).CombinedOutput(); err != nil {
			t.Fatalf("making the video: %v\n%s", err, out)
		}
	}
	srv := newFileServer(t, dir)

	for _, file := range []string{"show.m3u8", "film.mp4"} {
		t.Run(file, func(t *testing.T) {
			h := newHarness(t, srv+"/"+file)
			h.job.Filename = "Clip (0.03-0.06)"
			h.job.Clip = clip.Range{From: 3.2, To: 6.7}
			h.job.Subs, h.job.Sidecar = "en", true
			h.job.SubText = "1\n00:00:01,000 --> 00:00:02,000\nbefore\n\n2\n00:00:04,000 --> 00:00:05,000\nduring\n"
			if err := h.run(context.Background()); err != nil {
				t.Fatal(err)
			}
			path := h.rec.claimed()
			if filepath.Base(path) != "Clip (0.03-0.06).mp4" {
				t.Errorf("saved as %q", filepath.Base(path))
			}
			out, err := exec.Command(ffprobe, "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", path).Output()
			if err != nil {
				t.Fatalf("ffprobe: %v", err)
			}
			if d, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); math.Abs(d-3.5) > 0.1 {
				t.Errorf("the clip is %.2fs long, want 3.5s", d)
			}
			srt, err := os.ReadFile(strings.TrimSuffix(path, ".mp4") + ".en.srt")
			if want := "1\n00:00:00,800 --> 00:00:01,800\nduring\n\n"; err != nil || string(srt) != want {
				t.Errorf("captions %q (%v), want %q", srt, err, want)
			}
			info, _ := os.Stat(path)
			if got := h.rec.last(); info == nil || got.Done != info.Size() || got.Size != info.Size() {
				t.Errorf("final progress %+v", got)
			}
		})
	}
}

func TestAStoppedClipStartsOver(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("needs ffmpeg")
	}
	dir := t.TempDir()
	if out, err := exec.Command(ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=duration=30:size=640x360:rate=25",
		"-c:v", "libx264", "-g", "50", "-b:v", "2M", "-movflags", "+faststart", filepath.Join(dir, "film.mp4")).CombinedOutput(); err != nil {
		t.Fatalf("making the video: %v\n%s", err, out)
	}
	files := http.FileServer(http.Dir(dir))
	var slow atomic.Bool
	slow.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if slow.Load() {
			w = slowWriter{w}
		}
		files.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	h := newHarness(t, srv.URL+"/film.mp4")
	h.job.Filename = "Film (0.02-0.22)"
	h.job.Clip = clip.Range{From: 2, To: 22}
	ctx, cancel := context.WithCancel(context.Background())
	result := h.start(ctx)
	waitFor(t, "the clip to be under way", func() bool { return h.rec.last().Done > 0 })
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("stopped run returned %v", err)
	}
	slow.Store(false)
	if err := h.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", h.rec.claimed()).Output()
	if d, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); err != nil || math.Abs(d-20) > 0.1 {
		t.Errorf("the clip is %.2fs long, want 20s (%v)", d, err)
	}
}

type slowWriter struct{ http.ResponseWriter }

func (w slowWriter) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		n := min(len(p), 8<<10)
		time.Sleep(20 * time.Millisecond)
		m, err := w.ResponseWriter.Write(p[:n])
		written += m
		if err != nil {
			return written, err
		}
		p = p[n:]
	}
	return written, nil
}
