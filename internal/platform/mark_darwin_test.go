package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestMarkDownloadedQuarantinesTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.bin")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MarkDownloaded(path, "https://cdn.example.com/a.bin", "https://example.com/page"); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 256)
	n, err := unix.Getxattr(path, "com.apple.quarantine", got)
	if err != nil {
		t.Fatal(err)
	}
	if mark := string(got[:n]); !strings.HasPrefix(mark, "0081;") || !strings.HasSuffix(mark, ";Snag;") {
		t.Errorf("com.apple.quarantine = %q", mark)
	}
}
