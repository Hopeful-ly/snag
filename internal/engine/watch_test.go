package engine

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestAvailableFollowsADownloadThroughItsLife(t *testing.T) {
	te := newTestEngine(t, Settings{MaxActive: 1})
	if _, err := te.Available("nope", 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("an unknown download: %v", err)
	}

	d := te.add(t, AddRequest{URL: "https://example.com/film.mp4"})
	waiting := te.add(t, AddRequest{URL: "https://example.com/next.mp4"})
	d = te.waitRunning(t, d.ID)
	a, err := te.Available(d.ID, 4)
	if err != nil || a.Path != filepath.Join(te.workRoot, d.Work, "data.part") || a.N != 6 || a.Size != 100 || a.Done {
		t.Errorf("while it runs: %+v, %v", a, err)
	}
	if d.Playable != 10 {
		t.Errorf("playable = %d, want what the runner reported", d.Playable)
	}
	if a, err := te.Available(waiting.ID, 0); err != nil || a.Path != "" || a.N != 0 {
		t.Errorf("waiting to run: %+v, %v; want nothing to read yet", a, err)
	}

	te.runner.finish(d.URL, nil)
	d = te.waitFor(t, d.ID, StatusDone)
	if a, err := te.Available(d.ID, 40); err != nil || a.Path != d.Path || a.N != d.Size-40 || !a.Done {
		t.Errorf("finished: %+v, %v", a, err)
	}

	te.runner.finish(waiting.URL, errors.New("server said no"))
	te.waitFor(t, waiting.ID, StatusError)
	if _, err := te.Available(waiting.ID, 0); !errors.Is(err, ErrNotWatchable) {
		t.Errorf("failed: %v, want ErrNotWatchable", err)
	}
}
