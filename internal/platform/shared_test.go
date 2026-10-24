package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenSharedLetsTheFileBeRenamedAndDeleted(t *testing.T) {
	dir := t.TempDir()
	part := filepath.Join(dir, "data.part")
	if err := os.WriteFile(part, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := OpenShared(part)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	final := filepath.Join(dir, "film.mp4")
	if err := os.Rename(part, final); err != nil {
		t.Fatalf("renaming a file open for watching: %v", err)
	}
	buf := make([]byte, 5)
	if _, err := f.ReadAt(buf, 0); err != nil || string(buf) != "hello" {
		t.Errorf("reading after the rename: %q, %v", buf, err)
	}
	if err := os.Remove(final); err != nil {
		t.Errorf("deleting a file open for watching: %v", err)
	}
}
