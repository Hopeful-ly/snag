package probe

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Roham1299/snag/internal/tools"
)

// fakeYtDLP answers a listing with list and anything else with videoJSON, logging each run.
func fakeYtDLP(t *testing.T, list string) (path, runs string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in yt-dlp is a shell script")
	}
	dir := t.TempDir()
	runs = filepath.Join(dir, "runs")
	for name, text := range map[string]string{"list.json": list, "video.json": videoJSON} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	script := `#!/bin/sh
for last; do :; done
case "$*" in
*--flat-playlist*) echo "list $last" >> "` + runs + `"; cat "` + dir + `/list.json" ;;
*) echo "video $last" >> "` + runs + `"; cat "` + dir + `/video.json" ;;
esac
`
	path = filepath.Join(dir, "yt-dlp")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path, runs
}

func ranWith(t *testing.T, runs string) string {
	t.Helper()
	data, _ := os.ReadFile(runs)
	return strings.TrimSpace(string(data))
}

const (
	playlistJSON = `{"_type": "playlist", "title": "Season 1", "entries": [
		{"_type": "url", "id": "e1", "url": "https://v.example/watch?v=e1", "title": "One"},
		{"_type": "url", "id": "e2", "url": "https://v.example/watch?v=e2", "title": "Two"}]}`
	videoJSON = `{"title": "Two", "duration": 60, "extractor_key": "Example",
		"formats": [{"format_id": "1", "ext": "mp4", "vcodec": "avc1", "acodec": "mp4a", "height": 720}]}`
)

func TestProbeAVideoInAPlaylist(t *testing.T) {
	ytdlp, runs := fakeYtDLP(t, playlistJSON)
	p := New(tools.Paths{YtDLP: ytdlp})
	page := "https://v.example/watch?v=e2&list=s1"
	result := p.Probe(context.Background(), Request{URL: page})
	if !result.Supported || result.Title != "Two" || result.Playlist == nil ||
		*result.Playlist != (Playlist{Title: "Season 1", Count: 2}) {
		t.Fatalf("result %+v", result)
	}
	if got, want := ranWith(t, runs), "list "+page+"\nvideo "+page; got != want {
		t.Errorf("runs:\n%s\nwant:\n%s", got, want)
	}
	if list, err := p.List(context.Background(), Request{URL: page}); err != nil || len(list.Entries) != 2 {
		t.Errorf("List = %+v, %v", list, err)
	}
	if got := strings.Count(ranWith(t, runs), "list "); got != 1 {
		t.Errorf("listed %d times; the second should be remembered", got)
	}
}

func TestProbeAVideoInAListThatFails(t *testing.T) {
	ytdlp, runs := fakeYtDLP(t, "") // the listing prints nothing
	page := "https://v.example/watch?v=e2&list=private"
	result := New(tools.Paths{YtDLP: ytdlp}).Probe(context.Background(), Request{URL: page})
	if !result.Supported || result.Title != "Two" || result.Playlist != nil {
		t.Fatalf("result %+v", result)
	}
	if got, want := ranWith(t, runs), "list "+page+"\nvideo "+page; got != want {
		t.Errorf("runs:\n%s\nwant:\n%s", got, want)
	}
}

func TestProbeAPlaylistPage(t *testing.T) {
	ytdlp, runs := fakeYtDLP(t, playlistJSON)
	result := New(tools.Paths{YtDLP: ytdlp}).Probe(context.Background(), Request{URL: "https://v.example/playlist?list=s1"})
	if !result.Supported || result.Playlist == nil {
		t.Fatalf("result %+v", result)
	}
	if got := ranWith(t, runs); !strings.HasSuffix(got, "video https://v.example/watch?v=e1") {
		t.Errorf("runs:\n%s", got)
	}
}

func TestProbeAVideoOnItsOwn(t *testing.T) {
	ytdlp, runs := fakeYtDLP(t, videoJSON)
	result := New(tools.Paths{YtDLP: ytdlp}).Probe(context.Background(), Request{URL: "https://v.example/watch?v=e2"})
	if !result.Supported || result.Playlist != nil || len(result.Choices) == 0 {
		t.Fatalf("result %+v", result)
	}
	if got := ranWith(t, runs); got != "list https://v.example/watch?v=e2" {
		t.Errorf("runs:\n%s", got)
	}
}
