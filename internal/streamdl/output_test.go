package streamdl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/ytdlp"
)

// When resuming, yt-dlp may count only this run's bytes.
func TestProgressNeverDropsBelowWhatIsSaved(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "Clip.mp4")
	if err := os.WriteFile(current+".part", make([]byte, 8_118_272), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{dir: dir}
	s := &stream{job: engine.Job{WorkDir: dir}, report: rec, parallel: 8, estimate: sizeEstimate{parallel: 8}}
	s.handle(ytdlp.Parse("[download] Destination: " + current))
	s.handle(ytdlp.Parse("SNAGP downloading 2621440 NA NA 100000 NA hls-720 3 200"))

	if got := rec.last().Done; got != 8_118_272 {
		t.Errorf("done = %d, want the 8118272 bytes already saved", got)
	}
}

func TestSidecarDestKeepsTheLanguageSuffix(t *testing.T) {
	got := sidecarDest("/downloads/Cold Harbor.mp4", "Cold Harbor.en.srt")
	if got != "/downloads/Cold Harbor.en.srt" {
		t.Errorf("sidecarDest = %q", got)
	}
}
