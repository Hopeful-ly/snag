package watch

import (
	"bytes"
	"crypto/rand"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/engine"
)

// fakeDownload is a download whose file the test fills, renames and fails at will.
type fakeDownload struct {
	t    *testing.T
	data []byte

	mu      sync.Mutex
	d       engine.Download
	path    string
	written int64
	done    bool
	failed  error
	wants   []int64
}

func newFake(t *testing.T, size int) *fakeDownload {
	data := make([]byte, size)
	_, _ = rand.Read(data)
	f := &fakeDownload{
		t:    t,
		data: data,
		d:    engine.Download{ID: "d1", Filename: "film.mp4", Size: int64(size), Status: engine.StatusDownloading},
		path: filepath.Join(t.TempDir(), "data.part"),
	}
	if err := os.WriteFile(f.path, make([]byte, size), 0o644); err != nil { // sized up front, as Snag does
		t.Fatal(err)
	}
	return f
}

func (f *fakeDownload) write(n int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	file, err := os.OpenFile(f.path, os.O_WRONLY, 0)
	if err != nil {
		f.t.Error(err)
		return
	}
	defer file.Close()
	if _, err := file.WriteAt(f.data[f.written:n], f.written); err != nil {
		f.t.Error(err)
	}
	f.written = n
}

func (f *fakeDownload) finish() {
	f.write(int64(len(f.data)))
	f.mu.Lock()
	defer f.mu.Unlock()
	final := filepath.Join(filepath.Dir(f.path), "film.mp4")
	if err := os.Rename(f.path, final); err != nil {
		f.t.Error(err)
	}
	f.path, f.done = final, true
}

func (f *fakeDownload) fail(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed = err
}

func (f *fakeDownload) Get(id string) (engine.Download, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if id != f.d.ID {
		return engine.Download{}, engine.ErrNotFound
	}
	return f.d, nil
}

func (f *fakeDownload) Want(id string, off int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.wants = append(f.wants, off)
}

func (f *fakeDownload) wanted() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.wants)
}

func (f *fakeDownload) Available(id string, off int64) (engine.Available, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failed != nil {
		return engine.Available{}, f.failed
	}
	return engine.Available{Path: f.path, Size: f.d.Size, N: max(f.written-off, 0), Done: f.done}, nil
}

func start(t *testing.T, src Source) *Server {
	t.Helper()
	ffmpeg, _ := exec.LookPath("ffmpeg")
	s, err := Start(src, ffmpeg, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	s.wait = 5 * time.Millisecond
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestAPlayerGetsTheWholeFileAsItArrives(t *testing.T) {
	f := newFake(t, 3<<20)
	s := start(t, f)
	go func() {
		for n := int64(0); n < int64(len(f.data)); n += 256 << 10 {
			time.Sleep(5 * time.Millisecond)
			f.write(min(n+256<<10, int64(len(f.data))))
		}
		f.finish() // it finishes while the player is still reading
	}()

	resp, err := http.Get(s.URL(f.d))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	if err != nil || !bytes.Equal(got, f.data) {
		t.Fatalf("got %d bytes (%v), want the file's %d", len(got), err, len(f.data))
	}
	if ct := resp.Header.Get("Content-Type"); ct != "video/mp4" {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestARangeNotThereYetWaitsForIt(t *testing.T) {
	f := newFake(t, 1<<20)
	f.write(1000)
	s := start(t, f)

	type answer struct {
		status int
		rng    string
		body   []byte
		err    error
	}
	answered := make(chan answer, 1)
	go func() {
		req, _ := http.NewRequest(http.MethodGet, s.URL(f.d), nil)
		req.Header.Set("Range", "bytes=5000-5099")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			answered <- answer{err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		answered <- answer{resp.StatusCode, resp.Header.Get("Content-Range"), body, err}
	}()
	select {
	case <-answered:
		t.Fatal("answered before those bytes arrived")
	case <-time.After(100 * time.Millisecond):
	}
	if got := f.wanted(); !slices.Equal(got, []int64{5000}) {
		t.Errorf("asked the download for %v first, want [5000] once", got)
	}
	f.write(6000)
	select {
	case a := <-answered:
		if a.err != nil || a.status != http.StatusPartialContent || !bytes.Equal(a.body, f.data[5000:5100]) {
			t.Fatalf("status %d, %d bytes, %v", a.status, len(a.body), a.err)
		}
		if a.rng != "bytes 5000-5099/1048576" {
			t.Errorf("Content-Range = %q", a.rng)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("never answered once the bytes arrived")
	}
}

func TestAFailedDownloadEndsTheStream(t *testing.T) {
	f := newFake(t, 1<<20)
	f.write(100 << 10)
	s := start(t, f)
	go func() { time.Sleep(50 * time.Millisecond); f.fail(engine.ErrNotWatchable) }()

	resp, err := http.Get(s.URL(f.d))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	if err == nil || len(got) >= len(f.data) {
		t.Errorf("read %d bytes, %v; want the stream cut short", len(got), err)
	}
}

func TestHEADSaysWhatThereIs(t *testing.T) {
	f := newFake(t, 4096)
	s := start(t, f)
	resp, err := http.Head(s.URL(f.d))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.ContentLength != 4096 || resp.Header.Get("Accept-Ranges") != "bytes" {
		t.Errorf("status %d, length %d, ranges %q", resp.StatusCode, resp.ContentLength, resp.Header.Get("Accept-Ranges"))
	}
}

func TestOnlyThisComputerWithTheSecretGetsIn(t *testing.T) {
	f := newFake(t, 4096)
	f.write(4096)
	s := start(t, f)
	good := s.URL(f.d)
	status := func(method, url, host string) int {
		t.Helper()
		req, _ := http.NewRequest(method, url, nil)
		if host != "" {
			req.Host = host
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if got := status(http.MethodGet, good, ""); got != http.StatusOK {
		t.Fatalf("the right address: %d", got)
	}
	if got := status(http.MethodGet, strings.Replace(good, s.secret, strings.Repeat("0", len(s.secret)), 1), ""); got != http.StatusNotFound {
		t.Errorf("a wrong secret: %d", got)
	}
	if got := status(http.MethodGet, good, "evil.example:80"); got != http.StatusForbidden {
		t.Errorf("another host name (DNS rebinding): %d", got)
	}
	if got := status(http.MethodPost, good, ""); got != http.StatusMethodNotAllowed {
		t.Errorf("POST: %d", got)
	}
	if got := status(http.MethodGet, strings.Replace(good, "/d1/", "/d2/", 1), ""); got != http.StatusNotFound {
		t.Errorf("an unknown download: %d", got)
	}

	f.mu.Lock()
	f.d.Estimated = true
	f.mu.Unlock()
	if got := status(http.MethodGet, good, ""); got != http.StatusServiceUnavailable {
		t.Errorf("a size that is only a guess: %d", got)
	}
}

func TestClosingEndsWaitingStreams(t *testing.T) {
	f := newFake(t, 1<<20)
	s := start(t, f)
	ended := make(chan error, 1)
	go func() {
		resp, err := http.Get(s.URL(f.d))
		if err == nil {
			_, err = io.ReadAll(resp.Body)
			resp.Body.Close()
		}
		ended <- err
	}()
	time.Sleep(50 * time.Millisecond)
	_ = s.Close()
	select {
	case err := <-ended:
		if err == nil {
			t.Error("a stream waiting for bytes ended cleanly though nothing came")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a waiting stream kept going after Close")
	}
}
