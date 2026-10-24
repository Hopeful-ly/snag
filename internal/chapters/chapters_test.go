package chapters

import (
	"context"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Roham1299/snag/internal/clip"
)

func TestParse(t *testing.T) {
	got := Parse(`[{"start_time": 0, "title": "Intro", "end_time": 76}, {"start_time": 76, "title": "Rules", "end_time": 162.5}]`)
	want := []Chapter{{0, 76, "Intro"}, {76, 162.5, "Rules"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	for _, none := range []string{"NA", "null", ""} {
		if got := Parse(none); got != nil {
			t.Errorf("Parse(%q) = %+v, want none", none, got)
		}
	}
}

// Regression: a clip of 2:21.98–3:48.17 showed the whole video's chapters.
func TestCut(t *testing.T) {
	chs := []Chapter{{0, 76, "Intro"}, {76, 162, "Rules"}, {162, 201, "Load testing"}, {201, 233, "Submissions"}, {233, 304, "Ruby"}}
	got := Cut(chs, clip.Range{From: 141.98, To: 228.17})
	want := []Chapter{{0, 20.02, "Rules"}, {20.02, 59.02, "Load testing"}, {59.02, 86.19, "Submissions"}}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range got {
		if got[i].Title != want[i].Title || millis(got[i].Start) != millis(want[i].Start) || millis(got[i].End) != millis(want[i].End) {
			t.Errorf("chapter %d is %+v, want %+v", i, got[i], want[i])
		}
	}

	if got := Cut([]Chapter{{10, 0, "Last"}}, clip.Range{From: 12, To: 20}); !reflect.DeepEqual(got, []Chapter{{0, 8, "Last"}}) {
		t.Errorf("a last chapter without an end: %+v", got)
	}
	if got := Cut(chs, clip.Range{From: 400, To: 410}); got != nil {
		t.Errorf("a clip past every chapter got %+v", got)
	}
}

func TestEmbedReplacesTheChapters(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("needs ffmpeg")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("needs ffprobe")
	}
	video := filepath.Join(t.TempDir(), "clip.mp4")
	if out, err := exec.Command(ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=duration=4:size=160x120:rate=25",
		"-c:v", "libx264", video).CombinedOutput(); err != nil {
		t.Fatalf("making the video: %v\n%s", err, out)
	}
	chs := []Chapter{{0, 1.5, "Rules; part=1"}, {1.5, 4, "Load testing"}}
	if err := Embed(context.Background(), ffmpeg, video, chs); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(ffprobe, "-v", "error", "-show_entries", "chapter=start_time,end_time:chapter_tags=title",
		"-of", "csv=p=0", video).Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	want := "0.000000,1.500000,Rules; part=1\n1.500000,4.000000,Load testing"
	if got := strings.TrimSpace(string(out)); got != want {
		t.Errorf("chapters\n%s\nwant\n%s", got, want)
	}
}
