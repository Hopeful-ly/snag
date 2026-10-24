package streamdl

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/checksum"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/meta"
	"github.com/Roham1299/snag/internal/naming"
	"github.com/Roham1299/snag/internal/streamdl/streamtest"
	"github.com/Roham1299/snag/internal/tools"
)

func TestPauseStopsYtDLPAndResumeContinues(t *testing.T) {
	srv := streamtest.NewServer(t)
	h := newHarness(t, srv.URL+"/pause/v.m3u8")
	h.job.Filename = "Resume me"

	ctx, cancel := context.WithCancel(context.Background())
	result := h.start(ctx)
	waitFor(t, "10 pieces", func() bool { return len(srv.Completed("/pause/")) >= 10 })
	// A piece completing at the moment of the pause may not be saved.
	finished := srv.Completed("/pause/")
	time.Sleep(500 * time.Millisecond)
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("paused run returned %v", err)
		}
	case <-time.After(stopGrace + 5*time.Second):
		t.Fatal("yt-dlp didn't stop")
	}
	// yt-dlp counts fetched pieces not yet in its file, which a pause drops.
	before := keptBytes(t, h.job.WorkDir)
	asked := len(srv.Requested("/pause/"))
	intact := streamtest.StateIntact(h.job.WorkDir)

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	result = h.start(ctx)
	waitFor(t, "new pieces", func() bool { return len(srv.Requested("/pause/")) > asked+8 })
	cancel()
	<-result //nolint:errcheck // cancelled: the error is expected

	if !intact {
		t.Log("the pause cut yt-dlp's state short, so it started over by design; not checking for refetches")
		return
	}
	for _, again := range srv.Requested("/pause/")[asked:] {
		if slices.Contains(finished, again) {
			t.Errorf("fetched a finished piece again: %s", again)
			break
		}
	}
	if after := h.rec.last().Done; before == 0 || after < before {
		t.Errorf("%d bytes were kept, but progress continued from %d", before, after)
	}
}

func TestDownloadingStreamCanBeWatched(t *testing.T) {
	srv := streamtest.NewServer(t)
	srv.Delay = 20 * time.Millisecond
	h := newHarness(t, srv.URL+"/watch/v.m3u8")
	h.job.Filename = "Watch me"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := h.start(ctx)
	path := filepath.Join(h.downloads, "Watch me.mp4")
	waitFor(t, "the final name", func() bool { _, err := os.Stat(path); return err == nil })
	var sizes []int64
	for range 4 {
		time.Sleep(time.Second)
		if info, err := os.Stat(path); err == nil {
			sizes = append(sizes, info.Size())
		}
	}
	cancel()
	<-result //nolint:errcheck // cancelled: the error is expected
	if len(sizes) < 2 || sizes[len(sizes)-1] <= sizes[0] {
		t.Errorf("the file at its final name didn't grow: %v", sizes)
	}
}

func TestExpiredLinkSaysHowToContinue(t *testing.T) {
	srv := streamtest.NewServer(t)
	h := newHarness(t, srv.URL+"/tokA/v.m3u8")

	ctx, cancel := context.WithCancel(context.Background())
	result := h.start(ctx)
	waitFor(t, "10 pieces", func() bool { return len(srv.Completed("/tokA/")) >= 10 })
	cancel()
	<-result //nolint:errcheck // cancelled: the error is expected

	srv.Expire("tokA")
	err := h.run(context.Background())
	if !errors.Is(err, ErrLinkExpired) || !strings.Contains(err.Error(), "continues where it stopped") {
		t.Errorf("error = %v, want ErrLinkExpired", err)
	}
}

func TestFinishesARealStream(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("needs ffmpeg to make a real stream")
	}
	dir := t.TempDir()
	cmd := exec.Command(ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=duration=6:size=160x120:rate=10",
		"-f", "lavfi", "-i", "sine=duration=6", "-c:v", "libx264", "-g", "10", "-c:a", "aac",
		"-f", "hls", "-hls_time", "1", "-hls_playlist_type", "vod", filepath.Join(dir, "clip.m3u8"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("making the stream: %v\n%s", err, out)
	}
	srv := newFileServer(t, dir)

	h := newHarness(t, srv+"/clip.m3u8")
	if err := h.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	path := h.rec.claimed()
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		t.Fatalf("finished file %s: %v", path, err)
	}
	if filepath.Base(path) != "clip.mp4" || h.rec.title() != "clip" {
		t.Errorf("named %q, titled %q; want clip.mp4 from the stream's title", filepath.Base(path), h.rec.title())
	}
	if got := h.rec.last(); got.Size != info.Size() || got.Done != got.Size || got.Estimated {
		t.Errorf("final progress %+v for a %d-byte file", got, info.Size())
	}
	if _, err := os.Stat(h.job.WorkDir); !os.IsNotExist(err) {
		t.Error("work folder left behind")
	}
	// The MP4 remux happens while tags are written, and is reported as a stage, not as 99%.
	h.rec.mu.Lock()
	stages := slices.Clone(h.rec.stages)
	h.rec.mu.Unlock()
	if slices.Contains(stages, "Fixing the video's container") || !slices.Contains(stages, "Adding its details") ||
		stages[len(stages)-1] != "Moving it into place" {
		t.Errorf("stages = %q", stages)
	}
	if isMPEGTS(path) {
		t.Error("the finished file is still MPEG-TS")
	}
	if head, _ := os.ReadFile(path); len(head) < 8 || string(head[4:8]) != "ftyp" {
		t.Error("the finished file isn't an MP4")
	}
}

func TestASongGetsItsTagsAndCover(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("needs ffmpeg to make a song")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("needs ffprobe to read the song")
	}
	dir := t.TempDir()
	for _, args := range [][]string{
		{"-f", "lavfi", "-i", "sine=duration=2", "-metadata", "title=master", filepath.Join(dir, "song.m4a")},
		{"-f", "lavfi", "-i", "color=blue:s=48x48", "-frames:v", "1", filepath.Join(dir, "cover.jpg")},
	} {
		if out, err := exec.Command(ffmpeg, append([]string{"-loglevel", "error"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("making the song: %v\n%s", err, out)
		}
	}
	srv := newFileServer(t, dir)

	h := newHarness(t, srv+"/song.m4a")
	h.job.Audio, h.job.Filename = "mp3", "Daft Punk - Get Lucky"
	h.job.Meta = meta.Label{Tags: meta.Tags{Title: "Get Lucky", Artist: "Daft Punk", Album: "Random Access Memories",
		Track: 8, Date: "2013-05-20", Cover: srv + "/cover.jpg"}}
	h.job.Naming = naming.Naming{Template: "{n}. {track} - {artist} - {title} ({year})", Number: "03"}
	if err := h.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	path := h.rec.claimed()
	if name := filepath.Base(path); name != "03. 08 - Daft Punk - Get Lucky (2013).mp3" {
		t.Errorf("named %q", name)
	}
	out, err := exec.Command(ffprobe, "-v", "error", "-show_entries", "stream=codec_type,width:format_tags",
		"-of", "default=nw=1", path).CombinedOutput()
	if err != nil {
		t.Fatalf("reading %s: %v\n%s", path, err, out)
	}
	for _, want := range []string{"TAG:title=Get Lucky", "TAG:artist=Daft Punk", "TAG:album=Random Access Memories",
		"TAG:track=8", "codec_type=video\nwidth=48"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("%s lacks %q:\n%s", filepath.Base(path), want, out)
		}
	}
	if note := h.rec.noted(); note != "" {
		t.Errorf("noted %q", note)
	}
}

func TestFetchesPictureAndSoundAtOnce(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("needs ffmpeg to make a real stream")
	}
	dir := t.TempDir()
	cmd := exec.Command(ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=duration=20:size=160x120:rate=10",
		"-f", "lavfi", "-i", "sine=duration=20", "-map", "0:v", "-map", "1:a", "-c:v", "libx264", "-g", "10",
		"-c:a", "aac", "-f", "hls", "-hls_time", "1", "-hls_playlist_type", "vod", "-master_pl_name", "master.m3u8",
		"-var_stream_map", "v:0,agroup:aud a:0,agroup:aud,default:yes", filepath.Join(dir, "s%v", "index.m3u8"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("making the stream: %v\n%s", err, out)
	}
	// Pieces are slow so the server can see whether both parts are fetched at once.
	var mu sync.Mutex
	inFlight := map[string]int{} // by folder: s0 is the picture, s1 the sound
	both := false
	files := http.FileServer(http.Dir(dir))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".ts") {
			track := strings.Split(strings.Trim(r.URL.Path, "/"), "/")[0]
			mu.Lock()
			inFlight[track]++
			both = both || (inFlight["s0"] > 0 && inFlight["s1"] > 0)
			mu.Unlock()
			time.Sleep(150 * time.Millisecond)
			defer func() { mu.Lock(); inFlight[track]--; mu.Unlock() }()
		}
		files.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	h := newHarness(t, srv.URL+"/master.m3u8")
	h.job.Connections = 2
	h.job.Filename = "Apart"
	h.job.Meta = meta.Label{Defaults: meta.Tags{Album: "Clips"}}
	h.job.Naming = naming.Naming{Template: "{album} - {title}"}
	if err := h.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if !both {
		t.Error("the picture and the sound were never fetched at the same time")
	}
	mu.Unlock()
	path := h.rec.claimed()
	out, err := exec.Command(strings.TrimSuffix(ffmpeg, "ffmpeg")+"ffprobe", "-v", "error", "-show_entries",
		"stream=codec_type", "-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatalf("probing %s: %v", path, err)
	}
	if got := strings.Fields(string(out)); !slices.Contains(got, "video") || !slices.Contains(got, "audio") {
		t.Errorf("the finished file has %v, want the picture and the sound", got)
	}
	if name := filepath.Base(path); name != "Clips - master.mkv" {
		t.Errorf("named %q", name)
	}
	if got := h.rec.last(); got.Done != got.Size || got.Estimated {
		t.Errorf("final progress %+v", got)
	}
	if _, err := os.Stat(h.job.WorkDir); !os.IsNotExist(err) {
		t.Error("work folder left behind")
	}
}

// keptBytes sums yt-dlp's .part files in dir, excluding .part-Frag pieces.
func keptBytes(t *testing.T, dir string) int64 {
	t.Helper()
	parts, err := filepath.Glob(filepath.Join(dir, "*.part"))
	if err != nil {
		t.Fatal(err)
	}
	var kept int64
	for _, p := range parts {
		if info, err := os.Stat(p); err == nil {
			kept += info.Size()
		}
	}
	return kept
}

type harness struct {
	runner    *Runner
	job       engine.Job
	rec       *recorder
	downloads string
}

func newHarness(t *testing.T, url string) *harness {
	t.Helper()
	programs := tools.Find("")
	if programs.YtDLP == "" {
		t.Skip("needs yt-dlp: run `task tools`, or set SNAG_TOOLS")
	}
	root := t.TempDir()
	downloads := filepath.Join(root, "downloads")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		t.Fatal(err)
	}
	return &harness{
		runner:    New(programs),
		job:       engine.Job{URL: url, WorkDir: filepath.Join(root, "work", "1"), Connections: 8},
		rec:       &recorder{dir: downloads},
		downloads: downloads,
	}
}

func (h *harness) run(ctx context.Context) error {
	h.job.Path = h.rec.claimed() // as the engine does when a download continues
	return h.runner.Run(ctx, h.job, h.rec)
}

func (h *harness) start(ctx context.Context) <-chan error {
	h.job.Path = h.rec.claimed()
	result := make(chan error, 1)
	go func() { result <- h.runner.Run(ctx, h.job, h.rec) }()
	return result
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

type recorder struct {
	dir string

	mu            sync.Mutex
	path          string
	titled        string
	note          string
	progress      engine.Progress
	stages        []string
	stageProgress [][2]int64
}

func (r *recorder) ClaimPath(name string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.path = naming.UniquePath(r.dir, name, nil)
	return r.path, nil
}

func (r *recorder) Titled(title string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.titled = title
}

func (r *recorder) Progress(p engine.Progress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.progress = p
}

func (r *recorder) Readable(engine.Readable) {}

func (r *recorder) Checksum(checksum.Sum) {}

func (r *recorder) Stage(text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stages = append(r.stages, text)
}

func (r *recorder) Said(string) {}

func (r *recorder) StageProgress(done, of int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stageProgress = append(r.stageProgress, [2]int64{done, of})
}

func (r *recorder) Note(text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.note = text
}

func (r *recorder) last() engine.Progress {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.progress
}

func (r *recorder) claimed() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.path
}

func (r *recorder) noted() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.note
}

func (r *recorder) title() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.titled
}

func newFileServer(t *testing.T, dir string) string {
	srv := httptest.NewServer(http.FileServer(http.Dir(dir)))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestWithoutCaptionsNotes(t *testing.T) {
	rec := &recorder{}
	s := &stream{report: rec}
	s.withoutCaptions(errors.New("Unable to download video subtitles for 'en': HTTP Error 429: Too Many Requests"))
	if want := "Saved without subtitles: the site refused them (too many requests); turn captions on in the player first"; rec.note != want {
		t.Errorf("note %q, want %q", rec.note, want)
	}
	s.withoutCaptions(errors.New("the site has none in this language"))
	if want := "Saved without subtitles: the site has none in this language"; rec.note != want {
		t.Errorf("note %q, want %q", rec.note, want)
	}
}

func TestAskAgain(t *testing.T) {
	busy := errors.New("Unable to download video subtitles for 'en': HTTP Error 429: Too Many Requests")
	for _, c := range []struct {
		address string
		err     error
		want    bool
	}{
		{"https://vimeo.com/1", busy, true},
		{"https://www.youtube.com/watch?v=abc", busy, false},
		{"https://youtu.be/abc", busy, false},
		{"https://m.YouTube.com/shorts/abc", busy, false},
		{"https://notyoutube.com/abc", busy, true},
		{"https://vimeo.com/1", errors.New("HTTP Error 404: Not Found"), false},
	} {
		if got := askAgain(c.address, c.err); got != c.want {
			t.Errorf("askAgain(%q, %q) = %v, want %v", c.address, c.err, got, c.want)
		}
	}
}

func TestFixContainerPutsMPEGTSInAnMP4(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("needs ffmpeg")
	}
	dir := t.TempDir()
	final := filepath.Join(dir, "clip.mp4")
	cmd := exec.Command(ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=duration=4:size=160x120:rate=10",
		"-c:v", "libx264", "-f", "mpegts", final)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !isMPEGTS(final) {
		t.Fatal("the test stream isn't MPEG-TS")
	}
	rec := &recorder{dir: dir}
	s := &stream{job: engine.Job{WorkDir: dir}, report: rec, final: final}
	s.programs.FFmpeg = ffmpeg
	if err := s.fixContainer(context.Background()); err != nil {
		t.Fatal(err)
	}
	if isMPEGTS(final) {
		t.Fatal("still MPEG-TS")
	}
	if head, _ := os.ReadFile(final); string(head[4:8]) != "ftyp" {
		t.Fatal("not an MP4")
	}
	if !slices.Contains(rec.stages, "Fixing the video's container") {
		t.Errorf("stages = %q", rec.stages)
	}
	before, _ := os.Stat(final)
	if err := s.fixContainer(context.Background()); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.Stat(final); !after.ModTime().Equal(before.ModTime()) {
		t.Error("rewrote an MP4")
	}
}

func TestRewritingSaysHowFar(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, n int) {
		if err := os.WriteFile(filepath.Join(dir, name), make([]byte, n), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, ok := rewriting(dir); ok {
		t.Fatal("rewriting with nothing being written")
	}
	write("clip.mp4", 1000)
	write("clip.mp4.part-Frag3", 5000)
	write("clip.temp.mp4", 400)
	if source, written, ok := rewriting(dir); !ok || source != 1000 || written != 400 {
		t.Fatalf("rewriting = %d, %d, %v", source, written, ok)
	}
}
