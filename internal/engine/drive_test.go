package engine_test

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/httpdl"
	"github.com/Roham1299/snag/internal/httpdl/httpdltest"
	"github.com/Roham1299/snag/internal/platform"
	"github.com/Roham1299/snag/internal/store"
)

// A video saved to another drive than Snag's work folder can be watched while it downloads and is
// finished by a rename on that drive. GitHub's Windows runners have temp and source on different drives.
func TestDownloadToAnotherDriveCanBeWatched(t *testing.T) {
	root := t.TempDir()
	here, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	downloads, err := os.MkdirTemp(here, "drive-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(downloads) })
	if platform.SameDrive(root, downloads) {
		if runtime.GOOS == "windows" && os.Getenv("GITHUB_ACTIONS") == "true" {
			t.Fatal("GitHub's Windows machines have the source on D: and temporary files on C:; not any more?")
		}
		t.Skip("the temporary folder and the source are on one drive here")
	}

	e := engine.New(engine.Options{
		Store:     store.NewFile[[]engine.Download](filepath.Join(root, "list.json")),
		Runners:   map[engine.Kind]engine.Runner{engine.KindHTTP: httpdl.New()},
		WorkRoot:  filepath.Join(root, "work"),
		Settings:  engine.Settings{DownloadDir: downloads, Connections: 8, MaxActive: 1},
		Logger:    slog.New(slog.DiscardHandler),
		SameDrive: platform.SameDrive,
		Hide:      platform.Hide,
	})
	t.Cleanup(func() { e.Shutdown(t.Context()) })

	srv := httpdltest.NewServer(t)
	data := randomBytes(24 << 20)
	srv.SetDelay(30 * time.Millisecond)
	d, _, err := e.Add(engine.AddRequest{URL: srv.Add("/films/Holiday.mp4", data)})
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for d.Done < 4<<20 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		d, _ = e.Get(d.ID)
	}
	if d.Path == "" || d.Done == d.Size {
		t.Fatalf("couldn't catch it downloading: %+v", d)
	}
	player, err := os.Open(d.Path)
	if err != nil {
		t.Fatalf("the downloading video doesn't open by its name: %v", err)
	}
	start := make([]byte, 1<<20)
	_, err = io.ReadFull(player, start)
	player.Close()
	if err != nil || !bytes.Equal(start, data[:len(start)]) {
		t.Fatalf("its start doesn't play: %v", err)
	}
	if _, err := os.Stat(filepath.Join(downloads, ".snag")); err != nil {
		t.Errorf("its parts aren't on the download's drive: %v", err)
	}

	srv.SetDelay(0)
	for d.Status != engine.StatusDone && d.Status != engine.StatusError && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		d, _ = e.Get(d.ID)
	}
	if got, err := os.ReadFile(d.Path); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("finished %q (%s): %v", d.Status, d.Error, err)
	}
	if _, err := os.Stat(filepath.Join(downloads, ".snag")); !os.IsNotExist(err) {
		t.Errorf("the parts folder is left behind: %v", err)
	}
}
