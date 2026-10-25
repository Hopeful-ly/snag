package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/Roham1299/snag/internal/feed"
)

func serve(t *testing.T, body string) (url string, hits *atomic.Int32) {
	t.Helper()
	hits = new(atomic.Int32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/v1.2.0/snag_1.2.0_amd64.deb", hits
}

func sumOf(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func TestDownload(t *testing.T) {
	url, hits := serve(t, "the installer")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "snag_1.1.0_amd64.deb"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	f := feed.File{URL: url, SHA256: sumOf("the installer")}

	got, err := Download(context.Background(), f, dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "snag_1.2.0_amd64.deb") {
		t.Fatalf("downloaded to %s", got)
	}
	if data, _ := os.ReadFile(got); string(data) != "the installer" {
		t.Fatalf("downloaded %q", data)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("left %d files, want only the new installer", len(entries))
	}

	if _, err := Download(context.Background(), f, dir); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Errorf("fetched %d times, want the downloaded installer kept", hits.Load())
	}
}

func TestDownloadRefusesAnotherFile(t *testing.T) {
	url, _ := serve(t, "something else")
	dir := t.TempDir()
	if _, err := Download(context.Background(), feed.File{URL: url, SHA256: sumOf("the installer")}, dir); err == nil {
		t.Fatal("took a file whose SHA-256 isn't the feed's")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("left %d files behind", len(entries))
	}
}

func TestInstallRefusesAChangedFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "snag_1.2.0_amd64.deb")
	if err := os.WriteFile(file, []byte("changed"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Install(file, sumOf("the installer")); err == nil {
		t.Fatal("installed a file whose SHA-256 isn't the feed's")
	}
}

func TestForNeedsAnInstall(t *testing.T) {
	r := &feed.Release{Version: "1.2.0", Files: map[string]feed.File{Kind(): {URL: "https://example.com/x"}}}
	// Tests run from a build directory, never from an install Snag can update.
	if _, ok := For(r); ok {
		t.Error("a test binary counts as an install")
	}
	if _, ok := For(nil); ok {
		t.Error("found an installer without a release")
	}
}
