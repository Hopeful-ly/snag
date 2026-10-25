package platform

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestMarkDownloadedRecordsTheOrigin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.bin")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MarkDownloaded(path, "https://cdn.example.com/a.bin", "https://example.com/page"); err != nil {
		t.Fatal(err)
	}
	for attr, want := range map[string]string{originAttr: "https://cdn.example.com/a.bin", referrerAttr: "https://example.com/page"} {
		got := make([]byte, 256)
		n, err := unix.Getxattr(path, attr, got)
		if errors.Is(err, unix.ENOTSUP) {
			t.Skip("the temporary folder's file system has no user attributes")
		}
		if err != nil {
			t.Fatalf("%s: %v", attr, err)
		}
		if string(got[:n]) != want {
			t.Errorf("%s = %q, want %q", attr, got[:n], want)
		}
	}
}
