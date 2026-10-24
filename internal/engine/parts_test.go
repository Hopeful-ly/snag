package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPartsStayOnTheDownloadsDrive(t *testing.T) {
	te := newTestEngine(t, Settings{})
	te.otherDrive.Store(true)
	d := te.add(t, AddRequest{URL: "https://example.com/film.mp4", Filename: "film.mp4"})
	parts := filepath.Join(te.dir, partsFolder)
	if job := te.runner.waitJobs(t, 1)[0]; filepath.Dir(job.WorkDir) != parts {
		t.Fatalf("parts in %s, want the download folder's %s", job.WorkDir, partsFolder)
	}

	// Snag starts again: the download finds its parts where they were.
	te.Shutdown(t.Context())
	te.start(t)
	if job := te.runner.waitJobs(t, 2)[1]; filepath.Dir(job.WorkDir) != parts {
		t.Errorf("after a restart, parts in %s", job.WorkDir)
	}

	te.runner.finish(d.URL, nil)
	te.waitFor(t, d.ID, StatusDone)
	waitGoneFile(t, parts, "the empty parts folder")
}

// waitGoneFile waits for path to be deleted, as it is in the background.
func waitGoneFile(t *testing.T, path, what string) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return
		}
	}
	t.Errorf("%s was left behind: %s", what, path)
}

// A finished download whose runner couldn't delete its parts (a file was still open on Windows)
// has them deleted after it, and the parts folder with them.
func TestAFinishedDownloadsLeftoverPartsAreDeleted(t *testing.T) {
	te := newTestEngine(t, Settings{})
	te.otherDrive.Store(true)
	d := te.add(t, AddRequest{URL: "https://example.com/film.mp4"})
	te.waitRunning(t, d.ID)
	work := te.runner.jobs()[0].WorkDir
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "film.mp4.part"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	te.runner.finish(d.URL, nil)
	te.waitFor(t, d.ID, StatusDone)
	waitGoneFile(t, filepath.Join(te.dir, partsFolder), "the parts folder with a finished download's leftovers")
}

// Snag starting again deletes what a removal left in a parts folder, and the folder once empty,
// but keeps the parts of downloads still to finish.
func TestStartSweepsPartsFoldersNobodyOwns(t *testing.T) {
	te := newTestEngine(t, Settings{})
	te.otherDrive.Store(true)
	kept := te.add(t, AddRequest{URL: "https://example.com/kept.mp4", Paused: true})
	if err := te.Resume(kept.ID); err != nil {
		t.Fatal(err)
	}
	te.waitRunning(t, kept.ID)
	keptWork := te.runner.jobs()[0].WorkDir
	te.Shutdown(t.Context())

	parts := filepath.Join(te.dir, partsFolder)
	orphan := filepath.Join(parts, "0123456789abcdef0123456789abcdef")
	other := filepath.Join(parts, "not-snags")
	for _, dir := range []string{keptWork, orphan, other} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(orphan, "piece"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	te.start(t)
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Error("a work folder nobody owns was kept")
	}
	if _, err := os.Stat(keptWork); err != nil {
		t.Errorf("an unfinished download's parts were deleted: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("a folder Snag didn't make was deleted: %v", err)
	}
}

// Deleting a work folder is tried again while something holds it, as an open file does on
// Windows.
func TestRemoveWorkTriesAgain(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root deletes regardless of permissions")
	}
	removeRetry = 20 * time.Millisecond
	t.Cleanup(func() { removeRetry = 500 * time.Millisecond })
	parts := t.TempDir()
	work := filepath.Join(parts, "work")
	if err := os.MkdirAll(filepath.Join(work, "inner"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(work, 0o500); err != nil { // its contents can't be deleted for now
		t.Fatal(err)
	}
	go func() { time.Sleep(60 * time.Millisecond); _ = os.Chmod(work, 0o700) }()
	if err := removeWork(work); err != nil {
		t.Fatalf("removeWork gave up: %v", err)
	}
	if _, err := os.Stat(work); !os.IsNotExist(err) {
		t.Error("the work folder is still there")
	}
}

func TestRemovingADownloadTidiesItsPartsFolder(t *testing.T) {
	te := newTestEngine(t, Settings{})
	te.otherDrive.Store(true)
	d := te.add(t, AddRequest{URL: "https://example.com/film.mp4"})
	te.waitRunning(t, d.ID)
	work := te.runner.jobs()[0].WorkDir
	if err := os.MkdirAll(work, 0o700); err != nil { // as a runner does
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "piece"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := te.Remove(d.ID, false); err != nil {
		t.Fatal(err)
	}
	te.waitGone(t, d.ID)
	if _, err := os.Stat(filepath.Join(te.dir, partsFolder)); !os.IsNotExist(err) {
		t.Errorf("parts folder left behind: %v", err)
	}
}

func TestPartsOnTheSameDriveStayInTheWorkFolder(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.add(t, AddRequest{URL: "https://example.com/a.bin"})
	te.waitRunning(t, d.ID)
	if job := te.runner.jobs()[0]; filepath.Dir(job.WorkDir) != te.workRoot {
		t.Errorf("parts in %s, want the work folder", job.WorkDir)
	}
	if _, err := os.Stat(filepath.Join(te.dir, partsFolder)); !os.IsNotExist(err) {
		t.Error("a parts folder was made in the download folder")
	}
}

func TestADownloadStartedInTheWorkFolderStaysThere(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.add(t, AddRequest{URL: "https://example.com/a.bin"})
	te.waitRunning(t, d.ID)
	work := te.runner.jobs()[0].WorkDir
	if err := os.MkdirAll(work, 0o700); err != nil { // as a runner does
		t.Fatal(err)
	}
	if err := te.Pause(d.ID); err != nil {
		t.Fatal(err)
	}
	te.waitFor(t, d.ID, StatusPaused)

	te.otherDrive.Store(true) // say, its folder now looks like another drive
	if err := te.Resume(d.ID); err != nil {
		t.Fatal(err)
	}
	if again := te.runner.waitJobs(t, 2)[1].WorkDir; again != work {
		t.Errorf("resumed in %s, but its parts are in %s", again, work)
	}
}
