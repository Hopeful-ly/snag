package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSameDrive(t *testing.T) {
	dir := t.TempDir()
	if !SameDrive(dir, filepath.Join(dir, "not", "made", "yet")) {
		t.Error("a folder and one to be made inside it aren't on one drive")
	}
	if err := Hide(dir); err != nil {
		t.Errorf("hiding a folder: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("the hidden folder is gone: %v", err)
	}
}
