package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/httpdl/httpdltest"
	"github.com/Roham1299/snag/internal/platform"
	"github.com/Roham1299/snag/internal/tools"
)

// End to end through the engine, downloader and stream server, with the test as the player.
func TestAVideoStreamsWhileItDownloads(t *testing.T) {
	s := startService(t)

	srv := httpdltest.NewServer(t)
	data := make([]byte, 24<<20)
	_, _ = rand.Read(data)
	srv.SetDelay(60 * time.Millisecond)
	d, _, err := s.Engine.Add(engine.AddRequest{URL: srv.Add("/films/Holiday.mp4", data), Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	d = waitPlayable(t, s, d, 2<<20)

	resp, err := http.Get(s.watch.URL(d))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	// The player reads the start now; the rest comes as it downloads.
	start := make([]byte, 1<<20)
	if _, err := io.ReadFull(resp.Body, start); err != nil || !bytes.Equal(start, data[:len(start)]) {
		t.Fatalf("the start doesn't stream: %v", err)
	}
	srv.SetDelay(0)
	rest, err := io.ReadAll(resp.Body)
	if err != nil || !bytes.Equal(append(start, rest...), data) {
		t.Fatalf("streamed %d bytes in all (%v), want the file's %d", len(start)+len(rest), err, len(data))
	}
}

func startService(t *testing.T) *Service {
	t.Helper()
	return startServiceWith(t, tools.Paths{})
}

func startServiceWith(t *testing.T, programs tools.Paths) *Service {
	t.Helper()
	dir := t.TempDir()
	s, err := Start(Options{
		Paths: platform.Paths{
			Config: filepath.Join(dir, "settings.json"), State: filepath.Join(dir, "list.json"),
			Work: filepath.Join(dir, "work"), Downloads: filepath.Join(dir, "downloads"),
			Socket: testSocket(t, dir),
		},
		Tools:  programs,
		Logger: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close(t.Context()) })
	if s.watch == nil {
		t.Fatal("the stream server didn't start")
	}
	return s
}

// waitPlayable waits until n bytes of d are playable while it still downloads.
func waitPlayable(t *testing.T, s *Service, d engine.Download, n int64) engine.Download {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for d.Playable < n && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		d, _ = s.Engine.Get(d.ID)
	}
	if d.Playable < n || d.Status == engine.StatusDone {
		t.Fatalf("couldn't catch it downloading with its start in: %+v", d)
	}
	return d
}

// Uses real mpv, on an MP4 with its index at the end.
func TestAPlayerPlaysAVideoWhileItDownloads(t *testing.T) {
	mpv, err := exec.LookPath("mpv")
	ffmpeg, err2 := exec.LookPath("ffmpeg")
	if err != nil || err2 != nil {
		t.Skip("needs mpv and ffmpeg")
	}
	film := filepath.Join(t.TempDir(), "film.mp4")
	make := exec.Command(ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=duration=300:size=640x360:rate=25",
		"-f", "lavfi", "-i", "sine=duration=300", "-c:v", "libx264", "-preset", "ultrafast", "-b:v", "4M",
		"-c:a", "aac", film) // the index goes at the end, as many cameras and encoders put it
	if out, err := make.CombinedOutput(); err != nil {
		t.Fatalf("making a video: %v\n%s", err, out)
	}
	data, err := os.ReadFile(film)
	if err != nil {
		t.Fatal(err)
	}

	s := startService(t)
	srv := httpdltest.NewServer(t)
	srv.SetDelay(300 * time.Millisecond)
	d, _, err := s.Engine.Add(engine.AddRequest{URL: srv.Add("/films/Test pattern.mp4", data), Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	d = waitPlayable(t, s, d, 2<<20)

	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	play := exec.CommandContext(ctx, mpv, "--no-config", "--vo=null", "--ao=null", "--length=3",
		"--msg-level=all=error", s.watch.URL(d))
	if out, err := play.CombinedOutput(); err != nil {
		t.Fatalf("mpv couldn't play it: %v\n%s", err, out)
	}
	if d, _ = s.Engine.Get(d.ID); d.Status == engine.StatusDone {
		t.Fatal("the download finished before mpv was done, so it didn't play while downloading")
	}

	// Jumping four minutes in, far past what has downloaded: Snag fetches from there next.
	jump := exec.CommandContext(ctx, mpv, "--no-config", "--vo=null", "--ao=null", "--start=240", "--length=2",
		"--msg-level=all=error", s.watch.URL(d))
	if out, err := jump.CombinedOutput(); err != nil {
		t.Fatalf("mpv couldn't play from 4:00: %v\n%s", err, out)
	}
	// 4:00 is 80% into the video: had the download not jumped, that much would be in by now.
	if d, _ = s.Engine.Get(d.ID); d.Done > d.Size*7/10 {
		t.Errorf("mpv played from 4:00 only with %d of %d bytes in: the download didn't jump there", d.Done, d.Size)
	}
}

// Covers separate video and audio streams (as YouTube's are), joined on the way, and a single file.
func TestAStreamPlaysWhileItDownloads(t *testing.T) {
	programs := tools.Find("")
	mpv, err := exec.LookPath("mpv")
	ffmpeg, err2 := exec.LookPath("ffmpeg")
	if programs.YtDLP == "" || err != nil || err2 != nil {
		t.Skip("needs yt-dlp (task tools), mpv and ffmpeg")
	}
	programs.FFmpeg = ffmpeg
	dir := t.TempDir()
	// Three minutes in 2-second pieces, picture and sound apart, and the same as one file.
	makeHLS := func(out string, apart bool) {
		args := []string{"-loglevel", "error", "-f", "lavfi", "-i", "testsrc=duration=180:size=640x360:rate=25",
			"-f", "lavfi", "-i", "sine=duration=180", "-map", "0:v", "-map", "1:a", "-c:v", "libx264",
			"-preset", "ultrafast", "-b:v", "1M", "-c:a", "aac", "-f", "hls", "-hls_time", "2",
			"-hls_playlist_type", "vod"}
		if apart {
			args = append(args, "-master_pl_name", "master.m3u8", "-var_stream_map",
				"v:0,agroup:aud a:0,agroup:aud,default:yes", filepath.Join(out, "s%v", "index.m3u8"))
		} else {
			args = append(args, filepath.Join(out, "index.m3u8"))
		}
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
			t.Fatalf("making a stream: %v\n%s", err, out)
		}
	}
	makeHLS(filepath.Join(dir, "apart"), true)
	makeHLS(filepath.Join(dir, "together"), false)
	// Each piece takes three seconds; yt-dlp fetches many at once, so the stream still finishes
	// in seconds.
	files := http.FileServer(http.Dir(dir))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".ts") {
			time.Sleep(3 * time.Second)
		}
		files.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	s := startServiceWith(t, programs)
	for _, tc := range []struct{ name, url string }{
		{"picture and sound apart", srv.URL + "/apart/master.m3u8"},
		{"one file", srv.URL + "/together/index.m3u8"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, _, err := s.Engine.Add(engine.AddRequest{URL: tc.url, Kind: engine.KindStream,
				Format: "bv*+ba/b", Title: tc.name, Confirmed: true})
			if err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(90 * time.Second)
			for {
				a, err := s.Engine.Available(d.ID, 0)
				if err != nil {
					t.Fatal(err)
				}
				if a.Done {
					t.Fatal("it finished before it could be watched")
				}
				if a.N >= 1<<20 && (!a.Separate || a.Sound != "") {
					if tc.url == srv.URL+"/apart/master.m3u8" && !a.Separate {
						t.Fatal("the picture and sound weren't seen as apart")
					}
					if a.Separate && !strings.HasSuffix(a.Sound, ".part") {
						// The sound comes alongside the picture, so it is far from in yet.
						t.Fatalf("watchable only once the sound was complete: %+v", a)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("never watchable: %+v", a)
				}
				time.Sleep(100 * time.Millisecond)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
			defer cancel()
			play := exec.CommandContext(ctx, mpv, "--no-config", "--vo=null", "--ao=null", "--length=3",
				"--msg-level=all=error", s.watch.URL(d))
			if out, err := play.CombinedOutput(); err != nil {
				t.Fatalf("mpv couldn't play it: %v\n%s", err, out)
			}
			if d, _ = s.Engine.Get(d.ID); d.Status == engine.StatusDone {
				t.Error("the download finished before mpv was done, so it didn't play while downloading")
			}
			// Removing finishes in the background; wait for it.
			_ = s.Engine.Remove(d.ID, false)
			for gone := time.Now().Add(30 * time.Second); time.Now().Before(gone); time.Sleep(50 * time.Millisecond) {
				if _, err := s.Engine.Get(d.ID); err != nil {
					break
				}
			}
		})
	}
}

// testSocket is where a test's app listens: a named pipe on Windows, where sockets aren't files.
func testSocket(t *testing.T, dir string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(`\\.\pipe\snag-test-%d-%s`, os.Getpid(), strings.ReplaceAll(t.Name(), "/", "-"))
	}
	return filepath.Join(dir, "app.sock")
}
