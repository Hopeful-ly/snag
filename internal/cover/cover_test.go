package cover

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func ffmpegOrSkip(t *testing.T) (string, string) {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg isn't installed")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe isn't installed")
	}
	return ffmpeg, ffprobe
}

func run(t *testing.T, name string, args ...string) []byte {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return out
}

func TestEmbed(t *testing.T) {
	ffmpeg, ffprobe := ffmpegOrSkip(t)
	dir := t.TempDir()
	picture := filepath.Join(dir, "cover.jpg")
	run(t, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=red:s=64x64", "-frames:v", "1", picture)
	for _, ext := range []string{".mp3", ".m4a"} {
		song := filepath.Join(dir, "song"+ext)
		run(t, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "sine=d=1", "-metadata", "title=Get Lucky", song)
		if err := Embed(context.Background(), ffmpeg, song, picture); err != nil {
			t.Fatal(err)
		}
		var probed struct {
			Streams []struct {
				CodecType   string         `json:"codec_type"`
				Width       int            `json:"width"`
				Disposition map[string]int `json:"disposition"`
			} `json:"streams"`
			Format struct {
				Tags map[string]string `json:"tags"`
			} `json:"format"`
		}
		out := run(t, ffprobe, "-v", "error", "-of", "json", "-show_streams", "-show_format", song)
		if err := json.Unmarshal(out, &probed); err != nil {
			t.Fatal(err)
		}
		var hasCover bool
		for _, s := range probed.Streams {
			hasCover = hasCover || (s.CodecType == "video" && s.Width == 64 && s.Disposition["attached_pic"] == 1)
		}
		if !hasCover || len(probed.Streams) != 2 || probed.Format.Tags["title"] != "Get Lucky" {
			t.Errorf("%s after Embed: %s", ext, out)
		}
		if _, err := os.Stat(filepath.Join(dir, "song.cover"+ext)); !os.IsNotExist(err) {
			t.Errorf("%s: the file written aside is left behind", ext)
		}
	}
	if err := Embed(context.Background(), ffmpeg, filepath.Join(dir, "video.mp4"), picture); !errors.Is(err, ErrNotASong) {
		t.Errorf("a video: %v", err)
	}
}

func TestFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/c" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png"))
	}))
	defer srv.Close()
	dir := t.TempDir()
	path, err := Fetch(context.Background(), srv.Client(), srv.URL+"/c", dir)
	if err != nil || filepath.Base(path) != "snag-cover.png" {
		t.Fatalf("Fetch = %q, %v", path, err)
	}
	if b, _ := os.ReadFile(path); string(b) != "png" {
		t.Errorf("fetched %q", b)
	}
	if _, err := Fetch(context.Background(), srv.Client(), srv.URL+"/gone", dir); err == nil {
		t.Error("a missing picture was fetched")
	}
}
