package engine

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestClashFindsTakenNames(t *testing.T) {
	te := newTestEngine(t, Settings{Ask: true})
	d := te.add(t, AddRequest{URL: "https://example.com/report", Filename: "report.pdf"})
	clash := func(name string) *Clash {
		t.Helper()
		c, err := te.Clash(d.ID, te.dir, name)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	if c := clash(""); c != nil {
		t.Fatalf("a free name clashes: %+v", c)
	}
	write(t, filepath.Join(te.dir, "report.pdf"), "old")
	if c := clash(""); c == nil || c.Existing != "report.pdf" || c.InUse || c.KeepBoth != "report (1).pdf" {
		t.Errorf("clash = %+v", c)
	}
	write(t, filepath.Join(te.dir, "report (1).pdf"), "older")
	if c := clash("report.pdf"); c == nil || c.KeepBoth != "report (2).pdf" {
		t.Errorf("with (1) taken too: %+v", c)
	}
	if c := clash("other name.pdf"); c != nil {
		t.Errorf("another name clashes: %+v", c)
	}

	busy := te.add(t, AddRequest{URL: "https://example.com/busy", Filename: "busy.bin", Confirmed: true})
	te.waitRunning(t, busy.ID)
	if c := clash("busy.bin"); c == nil || !c.InUse {
		t.Errorf("a name another download saves to: %+v", c)
	}
	if err := te.Confirm(d.ID, ConfirmRequest{Filename: "busy.bin", Replace: true}); !errors.Is(err, ErrNameInUse) {
		t.Errorf("replacing another download's name: %v, want ErrNameInUse", err)
	}
}

func TestClashKnowsAStreamsContainersAreUndecided(t *testing.T) {
	te := newTestEngine(t, Settings{Ask: true})
	d := te.add(t, AddRequest{URL: "https://example.com/show.m3u8", Kind: KindStream, Title: "Show"})
	write(t, filepath.Join(te.dir, "Show.mkv"), "old")
	c, err := te.Clash(d.ID, te.dir, "Show.mp4")
	if err != nil || c == nil || c.Existing != "Show.mkv" || c.KeepBoth != "Show (1)" {
		t.Errorf("clash = %+v, %v", c, err)
	}
}

func TestReplaceKeepsTheOldFileUntilTheNewOneIsDone(t *testing.T) {
	te := newTestEngine(t, Settings{Ask: true})
	old := filepath.Join(te.dir, "a.bin")
	write(t, old, "old")
	d := te.add(t, AddRequest{URL: "https://example.com/a.bin"})
	if err := te.Confirm(d.ID, ConfirmRequest{Filename: "a.bin", Replace: true}); err != nil {
		t.Fatal(err)
	}
	d = te.waitRunning(t, d.ID)
	if d.Path != old {
		t.Fatalf("path = %q, want the replaced file's", d.Path)
	}
	if got := read(t, setAside(old)); got != "old" {
		t.Fatalf("the old file isn't set aside while it downloads: %q", got)
	}

	te.runner.finish(d.URL, nil)
	te.waitFor(t, d.ID, StatusDone)
	if got := read(t, old); got != "done" {
		t.Errorf("the file has %q, want the new download", got)
	}
	if _, err := os.Lstat(setAside(old)); !os.IsNotExist(err) {
		t.Error("the old file is still kept after the new one finished")
	}
}

func TestRemovingAReplacementPutsTheOldFileBack(t *testing.T) {
	te := newTestEngine(t, Settings{Ask: true})
	old := filepath.Join(te.dir, "a.bin")
	write(t, old, "old")
	d := te.add(t, AddRequest{URL: "https://example.com/a.bin"})
	if err := te.Confirm(d.ID, ConfirmRequest{Filename: "a.bin", Replace: true}); err != nil {
		t.Fatal(err)
	}
	te.waitRunning(t, d.ID)
	if err := te.Remove(d.ID, false); err != nil {
		t.Fatal(err)
	}
	te.waitGone(t, d.ID)
	if got := read(t, old); got != "old" {
		t.Errorf("the old file has %q after the replacement was removed", got)
	}
}

func TestKeepingBothLeavesTheOldFileAlone(t *testing.T) {
	te := newTestEngine(t, Settings{Ask: true})
	old := filepath.Join(te.dir, "a.bin")
	write(t, old, "old")
	d := te.add(t, AddRequest{URL: "https://example.com/a.bin"})
	if err := te.Confirm(d.ID, ConfirmRequest{Filename: "a.bin"}); err != nil {
		t.Fatal(err)
	}
	d = te.waitRunning(t, d.ID)
	if filepath.Base(d.Path) != "a (1).bin" || read(t, old) != "old" {
		t.Errorf("path %q, old file %q", d.Path, read(t, old))
	}
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
