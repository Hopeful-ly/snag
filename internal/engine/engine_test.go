package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/cookies"
	"github.com/Roham1299/snag/internal/meta"
)

func TestAddRunsAndFinishes(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.add(t, AddRequest{
		URL:     "https://example.com/files/a.bin",
		Cookies: []cookies.Cookie{{Name: "session", Value: "1", Domain: "example.com"}},
	})
	te.waitFor(t, d.ID, StatusDownloading)
	te.runner.finish(d.URL, nil)

	d = te.waitFor(t, d.ID, StatusDone)
	if d.Path != filepath.Join(te.dir, "a.bin") || d.Done != 100 || d.Finished.IsZero() || d.Cookies != nil {
		t.Errorf("finished download = %+v", d)
	}
}

func TestFinishedFilesAreMarkedAsDownloaded(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.add(t, AddRequest{URL: "https://example.com/files/a.bin", Page: "https://example.com/files/"})
	te.waitFor(t, d.ID, StatusDownloading)
	te.runner.finish(d.URL, nil)
	d = te.waitFor(t, d.ID, StatusDone)
	if want := []marked{{d.Path, d.URL, d.Page}}; !slices.Equal(te.marks(), want) {
		t.Errorf("marked %+v, want %+v", te.marks(), want)
	}

	// A drive that can't hold the mark doesn't make the download fail.
	te.markFails.Store(true)
	d = te.add(t, AddRequest{URL: "https://example.com/files/b.bin"})
	te.waitFor(t, d.ID, StatusDownloading)
	te.runner.finish(d.URL, nil)
	te.waitFor(t, d.ID, StatusDone)
}

func TestMetaReachesTheRunner(t *testing.T) {
	te := newTestEngine(t, Settings{})
	label := meta.Label{Tags: meta.Tags{Title: "Pilot", Show: "Lanterns", Season: 1, Episode: 1}, Only: true}
	d := te.add(t, AddRequest{URL: "https://example.com/files/a.bin", Meta: label})
	if job := te.runner.waitJobs(t, 1)[0]; job.Meta != label {
		t.Errorf("job labelled %+v, want %+v", job.Meta, label)
	}
	te.runner.finish(d.URL, nil)
	te.waitFor(t, d.ID, StatusDone)
}

func TestAddRefusesWhatItCantDownload(t *testing.T) {
	te := newTestEngine(t, Settings{})
	for _, url := range []string{"file:///etc/passwd", "ftp://example.com/a", "https://", "not a url"} {
		if _, _, err := te.Add(AddRequest{URL: url}); !errors.Is(err, ErrBadURL) {
			t.Errorf("Add(%q) error = %v, want ErrBadURL", url, err)
		}
	}
	if _, _, err := te.Add(AddRequest{URL: "https://example.com/a", Kind: "torrent"}); err == nil {
		t.Error("a download was accepted without a runner for it")
	}
}

func TestAddingTheSameAddressAgainReturnsTheDownload(t *testing.T) {
	te := newTestEngine(t, Settings{})
	first := te.add(t, AddRequest{URL: "https://example.com/twice.bin", Paused: true})

	again, duplicate, err := te.Add(AddRequest{URL: first.URL})
	if err != nil || !duplicate || again.ID != first.ID {
		t.Fatalf("second Add = %s, duplicate %v, err %v", again.ID, duplicate, err)
	}
	te.waitFor(t, first.ID, StatusDownloading) // a paused one is resumed
	if n := len(te.List()); n != 1 {
		t.Errorf("list has %d downloads, want 1", n)
	}
}

func TestAddManyReportsTheListOnce(t *testing.T) {
	te := newTestEngine(t, Settings{})
	events, unsubscribe := te.Subscribe()
	defer unsubscribe()

	added, already, err := te.AddMany([]AddRequest{
		{URL: "https://example.com/1.bin", Paused: true},
		{URL: "https://example.com/2.bin", Paused: true},
		{URL: "https://example.com/1.bin", Paused: true},
	})
	if err != nil || added != 2 || already != 1 || len(te.List()) != 2 {
		t.Fatalf("added %d, already %d, list %d, err %v", added, already, len(te.List()), err)
	}

	var kinds []EventKind
	deadline := time.After(time.Second)
	for {
		select {
		case ev := <-events:
			kinds = append(kinds, ev.Kind)
		case <-deadline:
			t.Fatalf("events %v, want one reset", kinds)
		}
		if len(kinds) >= 1 {
			break
		}
	}
	time.Sleep(30 * time.Millisecond)
	for {
		select {
		case ev := <-events:
			kinds = append(kinds, ev.Kind)
		default:
			if len(kinds) != 1 || kinds[0] != EventReset {
				t.Errorf("events %v, want one reset", kinds)
			}
			return
		}
	}
}

func TestAskWaitsForConfirm(t *testing.T) {
	te := newTestEngine(t, Settings{Ask: true})
	d := te.add(t, AddRequest{URL: "https://example.com/asked.bin"})
	if d.Status != StatusPending {
		t.Fatalf("status = %q, want pending", d.Status)
	}
	time.Sleep(50 * time.Millisecond)
	if len(te.runner.jobs()) != 0 {
		t.Fatal("a pending download started")
	}

	elsewhere := filepath.Join(t.TempDir(), "picked folder")
	if err := te.Confirm(d.ID, ConfirmRequest{Directory: elsewhere, Filename: "My name.bin"}); err != nil {
		t.Fatal(err)
	}
	te.waitFor(t, d.ID, StatusDownloading)
	te.runner.finish(d.URL, nil)
	d = te.waitFor(t, d.ID, StatusDone)
	if d.Path != filepath.Join(elsewhere, "My name.bin") {
		t.Errorf("path = %q", d.Path)
	}

	if err := te.Confirm(d.ID, ConfirmRequest{}); !errors.Is(err, ErrNotPending) {
		t.Errorf("confirming twice: %v, want ErrNotPending", err)
	}
	if err := te.Confirm("nope", ConfirmRequest{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("confirming an unknown download: %v, want ErrNotFound", err)
	}
}

func TestConfirmedAddSkipsAsking(t *testing.T) {
	te := newTestEngine(t, Settings{Ask: true})
	d := te.add(t, AddRequest{URL: "https://example.com/a.bin", Confirmed: true})
	te.waitFor(t, d.ID, StatusDownloading)
}

func TestRunsAtMostMaxActiveOldestFirst(t *testing.T) {
	te := newTestEngine(t, Settings{MaxActive: 2})
	a := te.add(t, AddRequest{URL: "https://example.com/a.bin"})
	b := te.add(t, AddRequest{URL: "https://example.com/b.bin"})
	c := te.add(t, AddRequest{URL: "https://example.com/c.bin"})

	te.waitFor(t, a.ID, StatusDownloading)
	te.waitFor(t, b.ID, StatusDownloading)
	if got, _ := te.Get(c.ID); got.Status != StatusQueued {
		t.Fatalf("third download is %q, want queued", got.Status)
	}
	te.runner.finish(a.URL, nil)
	te.waitFor(t, c.ID, StatusDownloading)
}

func TestPauseAndResume(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.add(t, AddRequest{URL: "https://example.com/a.bin"})
	te.waitRunning(t, d.ID)

	if err := te.Pause(d.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := te.Get(d.ID); got.Status != StatusPausing && got.Status != StatusPaused {
		t.Errorf("right after Pause: %q, want pausing", got.Status)
	}
	paused := te.waitFor(t, d.ID, StatusPaused)

	if err := te.Resume(d.ID); err != nil {
		t.Fatal(err)
	}
	jobs := te.runner.waitJobs(t, 2)
	if len(jobs) != 2 || jobs[1].Path != paused.Path || jobs[1].WorkDir != jobs[0].WorkDir {
		t.Errorf("resumed job = %+v, want the same path and work folder as before", jobs[len(jobs)-1])
	}
}

func TestFailedDownloadCanBeRetried(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.add(t, AddRequest{URL: "https://example.com/a.bin"})
	te.waitFor(t, d.ID, StatusDownloading)
	te.runner.finish(d.URL, errors.New("server said 404 Not Found"))

	d = te.waitFor(t, d.ID, StatusError)
	if d.Error != "server said 404 Not Found" {
		t.Errorf("error = %q", d.Error)
	}
	if err := te.Resume(d.ID); err != nil {
		t.Fatal(err)
	}
	if d = te.waitFor(t, d.ID, StatusDownloading); d.Error != "" {
		t.Errorf("error kept after retry: %q", d.Error)
	}
}

func TestRemoveAnswersAtOnceAndCleansUp(t *testing.T) {
	te := newTestEngine(t, Settings{})
	events, unsubscribe := te.Subscribe()
	defer unsubscribe()

	d := te.add(t, AddRequest{URL: "https://example.com/a.bin"})
	te.waitRunning(t, d.ID)
	work := te.runner.jobs()[0].WorkDir
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	if err := te.Remove(d.ID, false); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(started); took > 100*time.Millisecond {
		t.Errorf("Remove took %v", took)
	}
	te.waitGone(t, d.ID)
	if _, err := os.Stat(work); !os.IsNotExist(err) {
		t.Errorf("work folder left behind: %v", err)
	}

	var kinds []EventKind
	for len(events) > 0 {
		kinds = append(kinds, (<-events).Kind)
	}
	if len(kinds) < 3 || kinds[0] != EventAdded || kinds[len(kinds)-1] != EventRemoved {
		t.Errorf("events = %v, want added, updates, removed", kinds)
	}
}

func TestRemoveFinishedKeepsTheFileUnlessAsked(t *testing.T) {
	te := newTestEngine(t, Settings{})
	keep := te.add(t, AddRequest{URL: "https://example.com/keep.bin"})
	drop := te.add(t, AddRequest{URL: "https://example.com/drop.bin"})
	for _, d := range []Download{keep, drop} {
		te.waitFor(t, d.ID, StatusDownloading)
		te.runner.finish(d.URL, nil)
	}
	keep = te.waitFor(t, keep.ID, StatusDone)
	drop = te.waitFor(t, drop.ID, StatusDone)

	_ = te.Remove(keep.ID, false)
	_ = te.Remove(drop.ID, true)
	te.waitGone(t, keep.ID)
	te.waitGone(t, drop.ID)
	if _, err := os.Stat(keep.Path); err != nil {
		t.Errorf("kept file: %v", err)
	}
	if _, err := os.Stat(drop.Path); !os.IsNotExist(err) {
		t.Errorf("deleted file still there: %v", err)
	}
}

func TestSameNameGetsANumber(t *testing.T) {
	te := newTestEngine(t, Settings{})
	a := te.add(t, AddRequest{URL: "https://example.com/1", Filename: "same.bin"})
	b := te.add(t, AddRequest{URL: "https://example.com/2", Filename: "same.bin"})
	a = te.waitRunning(t, a.ID)
	b = te.waitRunning(t, b.ID)
	names := map[string]bool{filepath.Base(a.Path): true, filepath.Base(b.Path): true}
	if !names["same.bin"] || !names["same (1).bin"] {
		t.Errorf("names = %v", names)
	}
}

func TestRunningDownloadsResumeAfterRestart(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.add(t, AddRequest{URL: "https://example.com/a.bin"})
	d = te.waitRunning(t, d.ID)
	te.Shutdown(context.Background())

	te.start(t)
	jobs := te.runner.waitJobs(t, 2)
	if last := jobs[len(jobs)-1]; last.Path != d.Path {
		t.Errorf("after restart the job's path is %q, want %q", last.Path, d.Path)
	}
}

func TestLoadSettlesInterruptedStates(t *testing.T) {
	te := newTestEngine(t, Settings{})
	te.Shutdown(context.Background())

	work := filepath.Join(te.workRoot, "abc")
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatal(err)
	}
	saved := []Download{
		{ID: "pausing", URL: "https://example.com/1", Kind: KindHTTP, Status: StatusPausing},
		{ID: "removing", URL: "https://example.com/2", Kind: KindHTTP, Status: StatusRemoving, Work: "abc"},
		{ID: "unknown", URL: "https://example.com/3", Kind: "torrent", Status: StatusQueued},
	}
	if err := te.store.Save(saved); err != nil {
		t.Fatal(err)
	}
	te.start(t)

	if d, _ := te.Get("pausing"); d.Status != StatusPaused {
		t.Errorf("pausing became %q, want paused", d.Status)
	}
	if _, err := te.Get("removing"); !errors.Is(err, ErrNotFound) {
		t.Error("a download being removed is back")
	}
	if _, err := os.Stat(work); !os.IsNotExist(err) {
		t.Error("its work folder is still there")
	}
	if d, _ := te.Get("unknown"); d.Status != StatusError {
		t.Errorf("a download without a runner is %q, want error", d.Status)
	}
}

func TestRemovingKeepsAFinishedFileItsLeftoverPartStillLinksTo(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.add(t, AddRequest{URL: "https://example.com/clip.mp4"})
	te.waitRunning(t, d.ID)
	te.runner.finish(d.URL, nil)
	d = te.waitFor(t, d.ID, StatusDone)

	// As when a player kept the file open on Windows and the work folder couldn't be deleted.
	work := te.runner.jobs()[0].WorkDir
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(d.Path, filepath.Join(work, "data.part")); err != nil {
		t.Skip("no hard links here:", err)
	}

	_ = te.Remove(d.ID, false)
	te.waitGone(t, d.ID)
	if _, err := os.Stat(d.Path); err != nil {
		t.Errorf("finished file deleted: %v", err)
	}
}

func TestUnfinishedDownloadsLinkIsRemovedWithIt(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.add(t, AddRequest{URL: "https://example.com/clip.mp4"})
	d = te.waitRunning(t, d.ID)
	work := te.runner.jobs()[0].WorkDir
	part := filepath.Join(work, "data.part")
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(part, d.Path); err != nil {
		t.Skip("no hard links here:", err)
	}

	_ = te.Remove(d.ID, false)
	te.waitGone(t, d.ID)
	if _, err := os.Lstat(d.Path); !os.IsNotExist(err) {
		t.Errorf("the partial file is still at its final name: %v", err)
	}
}

func TestStartSweepsWorkFoldersNobodyOwns(t *testing.T) {
	te := newTestEngine(t, Settings{})
	te.Shutdown(context.Background())
	orphan := filepath.Join(te.workRoot, "0123456789abcdef")
	if err := os.MkdirAll(orphan, 0o700); err != nil {
		t.Fatal(err)
	}
	te.start(t)
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Error("orphaned work folder kept")
	}
}

func TestShutdownWaitsForAPausingDownload(t *testing.T) {
	te := newTestEngine(t, Settings{})
	te.runner.stopDelay = 300 * time.Millisecond
	d := te.add(t, AddRequest{URL: "https://example.com/a.bin"})
	te.waitRunning(t, d.ID)
	if err := te.Pause(d.ID); err != nil {
		t.Fatal(err)
	}
	te.Shutdown(context.Background())

	te.runner.mu.Lock()
	running := te.runner.running
	te.runner.mu.Unlock()
	if running != 0 {
		t.Errorf("%d runs still going after Shutdown", running)
	}
	if got, _ := te.Get(d.ID); got.Status != StatusPaused {
		t.Errorf("a paused download came back as %q, want paused", got.Status)
	}
}

func TestEverythingAtOnce(t *testing.T) {
	te := newTestEngine(t, Settings{MaxActive: 1})
	a := te.add(t, AddRequest{URL: "https://example.com/a.bin"})
	b := te.add(t, AddRequest{URL: "https://example.com/b.bin"})
	te.waitRunning(t, a.ID)
	te.PauseAll()
	te.waitFor(t, a.ID, StatusPaused)
	te.waitFor(t, b.ID, StatusPaused)

	te.ResumeAll()
	te.waitRunning(t, a.ID)
	te.runner.finish(a.URL, nil)
	te.waitFor(t, a.ID, StatusDone)
	te.waitRunning(t, b.ID)
	te.runner.finish(b.URL, errors.New("server said 404 Not Found"))
	te.waitFor(t, b.ID, StatusError)

	te.ResumeAll() // retries the failed one
	te.waitFor(t, b.ID, StatusDownloading)
	te.ClearDone()
	te.waitGone(t, a.ID)
	if _, err := te.Get(b.ID); err != nil {
		t.Error("ClearDone removed an unfinished download")
	}
}
