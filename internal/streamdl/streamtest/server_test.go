package streamtest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStateIntact(t *testing.T) {
	dir := t.TempDir()
	state := filepath.Join(dir, "Clip.mp4.ytdl")
	if err := os.WriteFile(state, []byte(`{"downloader": {"current_fragment": {"index": 12}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !StateIntact(dir) {
		t.Error("a readable state counted as cut short")
	}
	if err := os.WriteFile(state, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if StateIntact(dir) {
		t.Error("an emptied state counted as intact")
	}
}
