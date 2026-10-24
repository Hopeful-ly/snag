package watchlink

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMakeUsesAHardLink(t *testing.T) {
	work, final := setup(t)
	if !Make(final, work) {
		t.Fatal("no link made")
	}
	if !IsHard(final, work) {
		t.Error("not a hard link")
	}
	appendTo(t, work, "more")
	if got, _ := os.ReadFile(final); string(got) != "startmore" {
		t.Errorf("final name shows %q, want the growing file", got)
	}
	if !Make(final, work) {
		t.Error("making it again failed")
	}
}

func TestMakeFallsBackToASymlink(t *testing.T) {
	if !canSymlink(t) {
		t.Skip("this OS doesn't allow symbolic links here")
	}
	hardLink = func(string, string) error { return errors.New("different drive") }
	t.Cleanup(func() { hardLink = os.Link })

	work, final := setup(t)
	if !Make(final, work) || IsHard(final, work) || !Shows(final, work) {
		t.Fatal("no symbolic link made")
	}
	if err := Finish(final, work, true); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(final); err != nil || !info.Mode().IsRegular() {
		t.Errorf("finished file isn't a plain file: %v", err)
	}
}

func TestFinishKeepsAHardLink(t *testing.T) {
	work, final := setup(t)
	Make(final, work)
	if err := Finish(final, work, true); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(final); string(got) != "start" {
		t.Errorf("final file = %q", got)
	}
}

// ffmpeg rewrites a stream's file when it fixes it up, so the link shows the old one.
func TestFinishReplacesAStaleLink(t *testing.T) {
	work, final := setup(t)
	Make(final, work)
	rewritten := work + ".new"
	if err := os.WriteFile(rewritten, []byte("fixed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(rewritten, work); err != nil {
		t.Fatal(err)
	}
	if err := Finish(final, work, true); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(final); string(got) != "fixed" {
		t.Errorf("final file = %q, want the rewritten one", got)
	}
}

func TestFinishMovesWhenNothingWasLinked(t *testing.T) {
	work, final := setup(t)
	if err := Finish(final, work, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(work); !os.IsNotExist(err) {
		t.Error("work file still there")
	}
	if got, _ := os.ReadFile(final); string(got) != "start" {
		t.Errorf("final file = %q", got)
	}
}

func TestMakeNeverReplacesAnotherFile(t *testing.T) {
	work, final := setup(t)
	if err := os.WriteFile(final, []byte("someone else's"), 0o644); err != nil {
		t.Fatal(err)
	}
	if Make(final, work) {
		t.Error("claimed to link over a real file")
	}
	if got, _ := os.ReadFile(final); string(got) != "someone else's" {
		t.Errorf("file replaced: %q", got)
	}
}

func TestRemoveInto(t *testing.T) {
	work, final := setup(t)
	dir := filepath.Dir(work)
	Make(final, work)
	if err := RemoveInto(final, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(final); !os.IsNotExist(err) {
		t.Error("link into the work folder not removed")
	}

	other := filepath.Join(t.TempDir(), "other.mp4")
	if err := os.WriteFile(other, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RemoveInto(other, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Error("removed a file that wasn't a link into the work folder")
	}
}

func setup(t *testing.T) (work, final string) {
	t.Helper()
	root := t.TempDir()
	work = filepath.Join(root, "work", "data.part")
	if err := os.MkdirAll(filepath.Dir(work), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(work, []byte("start"), 0o644); err != nil {
		t.Fatal(err)
	}
	return work, filepath.Join(root, "clip.mp4")
}

func appendTo(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}

func canSymlink(t *testing.T) bool {
	dir := t.TempDir()
	return os.Symlink(filepath.Join(dir, "target"), filepath.Join(dir, "link")) == nil
}
