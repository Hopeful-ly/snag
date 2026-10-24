package engine

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/store"
)

// talker is a runner that says things and goes through a stage before it finishes.
type talker struct {
	staged chan struct{} // closed once it is in its stage
	finish chan struct{}
}

func (r talker) Run(ctx context.Context, job Job, report Reporter) error {
	path, err := report.ClaimPath("a.bin")
	if err != nil {
		return err
	}
	report.Progress(Progress{Size: 100, Done: 99, Speed: 5000, Pieces: 100, PiecesDone: 99})
	report.Said("[download] 99.0% of ~100B")
	report.Stage("Fixing the video's container")
	report.StageProgress(40, 100)
	close(r.staged)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.finish:
	}
	return writeFile(path)
}

func TestStagesAndActivity(t *testing.T) {
	r := talker{staged: make(chan struct{}), finish: make(chan struct{})}
	dir := t.TempDir()
	e := New(Options{
		Store:    store.NewFile[[]Download](filepath.Join(dir, "list.json")),
		Runners:  map[Kind]Runner{KindHTTP: r},
		WorkRoot: filepath.Join(dir, "work"),
		Settings: Settings{DownloadDir: filepath.Join(dir, "dl"), MaxActive: 1},
		Logger:   slog.New(slog.DiscardHandler),
	})
	t.Cleanup(func() { e.Shutdown(context.Background()) })
	d, _, err := e.Add(AddRequest{URL: "https://example.com/a.bin"})
	if err != nil {
		t.Fatal(err)
	}
	<-r.staged
	got, _ := e.Get(d.ID)
	if got.Stage != "Fixing the video's container" || got.StageDone != 40 || got.StageOf != 100 ||
		got.Speed != 0 || got.PiecesDone != 99 || got.Heard.IsZero() {
		t.Fatalf("while finishing: %+v", got)
	}
	close(r.finish)
	deadline := time.Now().Add(5 * time.Second)
	for got.Status != StatusDone && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		got, _ = e.Get(d.ID)
	}
	if got.Stage != "" || got.Pieces != 0 {
		t.Errorf("finished with stage %q, %d pieces", got.Stage, got.Pieces)
	}
	said, err := e.Activity(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, s := range said {
		lines = append(lines, s.Text)
	}
	want := "Started | [download] 99.0% of ~100B | Fixing the video's container… | Finished"
	if strings.Join(lines, " | ") != want {
		t.Errorf("activity = %q", strings.Join(lines, " | "))
	}
	if _, err := e.Activity("nope"); err == nil {
		t.Error("activity of nothing")
	}
}

func writeFile(path string) error {
	return os.WriteFile(path, []byte("done"), 0o644)
}
