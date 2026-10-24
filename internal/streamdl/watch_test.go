package streamdl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/ytdlp"
)

// Fetched apart, a video is watchable once both parts have their start, not once the sound is in.
func TestAVideoFetchedApartSaysWhenItCanBeWatched(t *testing.T) {
	dir := t.TempDir()
	rec := &recorder{dir: dir}
	s := &stream{job: engine.Job{WorkDir: dir}, report: rec, parallel: 8}
	picture := s.newPart(ytdlp.Format{FormatID: "137", VCodec: "avc1", Filesize: 100_000_000})
	sound := s.newPart(ytdlp.Format{FormatID: "140", VCodec: "none", Filesize: 24_000_000})
	s.parts = []*part{picture, sound}
	pictureFile, soundFile := filepath.Join(dir, "film.f137.mp4"), filepath.Join(dir, "film.f140.m4a")
	write := func(path string, n int) {
		t.Helper()
		if err := os.WriteFile(path, make([]byte, n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	line := func(p *part, text string) { p.handle(ytdlp.Parse(text)) }

	line(picture, "[download] Destination: "+pictureFile)
	line(sound, "[download] Destination: "+soundFile)
	write(pictureFile+".part", 600_000)
	write(soundFile+".part", 100_000)
	line(picture, "SNAGP downloading 600000 NA NA 150000 NA 137 NA NA")
	line(sound, "SNAGP downloading 100000 NA NA 90000 NA 140 NA NA")
	p := rec.last()
	if p.Size != 124_000_000 || !p.Estimated || p.Done != 700_000 || p.Speed != 240_000 {
		t.Errorf("progress %+v; want both parts' size, done and speed together", p)
	}
	if p.Playable != 0 || p.ToPlay != playStart-600_000+soundStart-100_000 {
		t.Errorf("playable %d, to play %d; want the rest of both starts", p.Playable, p.ToPlay)
	}
	if path, separate := s.Sound(); !separate || path != soundFile+".part" {
		t.Errorf("sound = %q, %v; want the sound as it fills", path, separate)
	}

	write(soundFile+".part", soundStart)
	line(sound, "SNAGP downloading 262144 NA NA 90000 NA 140 NA NA")
	if p := rec.last(); p.Playable != 600_000 || p.ToPlay != playStart-600_000 {
		t.Errorf("with the sound's start in: playable %d, to play %d", p.Playable, p.ToPlay)
	}
	if s.Path() != pictureFile+".part" {
		t.Errorf("path = %q, want the picture's", s.Path())
	}

	// The joining run deletes both parts: still nothing to wait for.
	if err := os.Rename(soundFile+".part", soundFile); err != nil {
		t.Fatal(err)
	}
	line(sound, "SNAGP finished 262144 262144 NA NA NA 140 NA NA")
	if err := os.Remove(soundFile); err != nil {
		t.Fatal(err)
	}
	write(pictureFile+".part", 2_000_000)
	line(picture, "SNAGP downloading 2000000 NA NA 150000 NA 137 NA NA")
	if p := rec.last(); p.ToPlay != 0 || p.Playable != 2_000_000 || p.Size != 100_262_144 || p.Speed != 150_000 {
		t.Errorf("with the sound done: %+v", p)
	}
}

func TestOneFileSaysWhenItCanBeWatched(t *testing.T) {
	dir := t.TempDir()
	rec := &recorder{dir: dir}
	s := &stream{job: engine.Job{WorkDir: dir}, report: rec, parallel: 1}
	file := filepath.Join(dir, "clip.mp4")
	s.handle(ytdlp.Parse("[download] Destination: " + file))
	if err := os.WriteFile(file+".part", make([]byte, 300_000), 0o644); err != nil {
		t.Fatal(err)
	}
	s.handle(ytdlp.Parse("SNAGP downloading 300000 NA NA NA NA hls-1 NA NA"))
	if p := rec.last(); p.Playable != 300_000 || p.ToPlay != playStart-300_000 {
		t.Errorf("playable %d, to play %d", p.Playable, p.ToPlay)
	}
	if _, separate := s.Sound(); separate {
		t.Error("one file was taken for parts")
	}
}
