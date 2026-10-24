package engine_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/httpdl"
	"github.com/Roham1299/snag/internal/httpdl/httpdltest"
	"github.com/Roham1299/snag/internal/store"
	"github.com/Roham1299/snag/internal/streamdl"
	"github.com/Roham1299/snag/internal/streamdl/streamtest"
	"github.com/Roham1299/snag/internal/tools"
)

// These tests run the engine with the real runners against local servers.

func TestDownloadEndToEnd(t *testing.T) {
	srv := httpdltest.NewServer(t)
	data := randomBytes(5 << 20)
	url := srv.Add("/files/Nivalis-Nights.zip", data, httpdltest.ContentType("application/zip"))
	app := newApp(t)

	first := app.add(t, url)
	first = app.waitFor(t, first.ID, engine.StatusDone, engine.StatusError)
	checkFile(t, first, "Nivalis-Nights.zip", data)

	// A finished download doesn't count as a duplicate: adding it again makes a second copy.
	second := app.add(t, url)
	second = app.waitFor(t, second.ID, engine.StatusDone, engine.StatusError)
	checkFile(t, second, "Nivalis-Nights (1).zip", data)
}

func TestDownloadResumesAfterRestart(t *testing.T) {
	srv := httpdltest.NewServer(t)
	data := randomBytes(12 << 20)
	url := srv.Add("/big.zip", data, httpdltest.ContentType("application/zip"))
	srv.SetDelay(40 * time.Millisecond)
	app := newApp(t)

	d := app.add(t, url)
	deadline := time.Now().Add(20 * time.Second)
	for d.Done < 2<<20 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		d, _ = app.Get(d.ID)
	}
	app.Shutdown(context.Background())

	srv.SetDelay(0)
	srv.ClearRanges()
	app.start(t)
	d = app.waitFor(t, d.ID, engine.StatusDone, engine.StatusError)
	checkFile(t, d, "big.zip", data)
	for _, r := range srv.Ranges() {
		if strings.HasPrefix(r, "bytes=0-") {
			t.Fatalf("started over after the restart: %v", srv.Ranges())
		}
	}
}

func TestServerErrorShowsOnTheDownload(t *testing.T) {
	srv := httpdltest.NewServer(t)
	app := newApp(t)
	d := app.add(t, srv.URL+"/missing.mp4")
	d = app.waitFor(t, d.ID, engine.StatusDone, engine.StatusError)
	if d.Status != engine.StatusError || !strings.Contains(d.Error, "404") {
		t.Errorf("status %q, error %q; want an error mentioning 404", d.Status, d.Error)
	}
}

func TestExpiredStreamContinuesUnderANewLink(t *testing.T) {
	if tools.Find("").YtDLP == "" {
		t.Skip("needs yt-dlp: run `task tools`, or set SNAG_TOOLS")
	}
	srv := streamtest.NewServer(t)
	app := newApp(t)
	first := engine.AddRequest{URL: srv.URL + "/tokA/v.m3u8", Kind: engine.KindStream,
		Page: "https://example.test/watch/1", Title: "Token test"}
	d, _, err := app.Add(first)
	if err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool { return len(srv.Completed("/tokA/")) >= 10 })
	if err := app.Pause(d.ID); err != nil {
		t.Fatal(err)
	}
	app.waitFor(t, d.ID, engine.StatusPaused)
	intact := streamtest.StateIntact(filepath.Join(app.root, "work", d.Work))

	srv.Expire("tokA")
	if err := app.Resume(d.ID); err != nil {
		t.Fatal(err)
	}
	failed := app.waitFor(t, d.ID, engine.StatusError, engine.StatusDone)
	if !strings.Contains(failed.Error, "continues where it stopped") {
		t.Fatalf("error %q, want the expired-link explanation", failed.Error)
	}

	renewed := first
	renewed.URL = srv.URL + "/tokB/v.m3u8"
	again, duplicate, err := app.Add(renewed)
	if err != nil || !duplicate || again.ID != d.ID {
		t.Fatalf("adding it again: %s, duplicate %v, err %v; want the old download", again.ID, duplicate, err)
	}
	waitUntil(t, func() bool { return len(srv.Requested("/tokB/")) >= 3 })
	_ = app.Pause(d.ID)
	if !intact {
		t.Log("the pause cut yt-dlp's state short, so it started over by design; not checking for refetches")
		return
	}
	first8 := []string{"/tokB/seg0.ts", "/tokB/seg1.ts", "/tokB/seg2.ts", "/tokB/seg3.ts",
		"/tokB/seg4.ts", "/tokB/seg5.ts", "/tokB/seg6.ts", "/tokB/seg7.ts"}
	for _, piece := range srv.Requested("/tokB/") {
		if slices.Contains(first8, piece) {
			t.Fatalf("started over under the new link: %v", srv.Requested("/tokB/"))
		}
	}
}

func waitUntil(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

type app struct {
	*engine.Engine
	root string
}

func newApp(t *testing.T) *app {
	a := &app{root: t.TempDir()}
	a.start(t)
	return a
}

func (a *app) start(t *testing.T) {
	a.Engine = engine.New(engine.Options{
		Store: store.NewFile[[]engine.Download](filepath.Join(a.root, "downloads.json")),
		Runners: map[engine.Kind]engine.Runner{
			engine.KindHTTP:   httpdl.New(),
			engine.KindStream: streamdl.New(tools.Find("")),
		},
		WorkRoot: filepath.Join(a.root, "work"),
		Settings: engine.Settings{DownloadDir: filepath.Join(a.root, "dl"), Connections: 8, MaxActive: 3},
		Logger:   slog.New(slog.DiscardHandler),
	})
	t.Cleanup(func() { a.Shutdown(context.Background()) })
}

func (a *app) add(t *testing.T, url string) engine.Download {
	t.Helper()
	d, _, err := a.Add(engine.AddRequest{URL: url})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func (a *app) waitFor(t *testing.T, id string, statuses ...engine.Status) engine.Download {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		d, err := a.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range statuses {
			if d.Status == s {
				return d
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("download %s never reached %v", id, statuses)
	return engine.Download{}
}

func checkFile(t *testing.T, d engine.Download, name string, want []byte) {
	t.Helper()
	if d.Status != engine.StatusDone {
		t.Fatalf("status %q, error %q", d.Status, d.Error)
	}
	if filepath.Base(d.Path) != name {
		t.Errorf("named %q, want %q", filepath.Base(d.Path), name)
	}
	got, err := os.ReadFile(d.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from what was served", name)
	}
	if d.Size != int64(len(want)) || d.Done != d.Size {
		t.Errorf("size %d, done %d; want %d", d.Size, d.Done, len(want))
	}
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}
