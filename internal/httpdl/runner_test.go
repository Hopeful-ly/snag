package httpdl

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/checksum"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/httpdl/httpdltest"
	"github.com/Roham1299/snag/internal/naming"
	"github.com/Roham1299/snag/internal/ratelimit"
)

func TestSegmentedDownload(t *testing.T) {
	srv := httpdltest.NewServer(t)
	data := randomBytes(12<<20 + 12345)
	url := srv.Add("/files/Nivalis-Nights.zip", data, httpdltest.ContentType("application/zip"))

	h := newHarness(t, url)
	if err := h.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	h.checkFile(t, "Nivalis-Nights.zip", data)
	if starts := rangeStarts(srv.Ranges()); len(starts) < 4 {
		t.Errorf("used %d connections, want at least 4: %v", len(starts), srv.Ranges())
	}
	if entries, _ := os.ReadDir(h.downloads); len(entries) != 1 {
		t.Errorf("download folder has %d entries, want just the file", len(entries))
	}
	if _, err := os.Stat(h.job.WorkDir); !os.IsNotExist(err) {
		t.Error("work folder left behind")
	}
}

func TestNames(t *testing.T) {
	srv := httpdltest.NewServer(t)
	data := randomBytes(1000)
	tests := []struct {
		name     string
		url      string
		filename string
		title    string
		want     string
	}{
		{"generic name from the address", srv.Add("/video.mp4", data), "", "", "video.mp4"},
		{"server's name", srv.Add("/named", data, httpdltest.Filename("Holiday clip.mp4")), "", "", "Holiday clip.mp4"},
		{"page title over a generic name", srv.Add("/stream/video.mp4", data), "", "Big: Trip?", "Big Trip.mp4"},
		{"user's name over everything", srv.Add("/named2", data, httpdltest.Filename("x.mp4")), "mine.mp4", "", "mine.mp4"},
		{"extension from the content type", srv.Add("/get", data, httpdltest.ContentType("application/pdf")), "", "Report", "Report.pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, tt.url)
			h.job.Filename, h.job.Title = tt.filename, tt.title
			if err := h.run(context.Background()); err != nil {
				t.Fatal(err)
			}
			h.checkFile(t, tt.want, data)
		})
	}
}

func TestPauseAndResume(t *testing.T) {
	srv := httpdltest.NewServer(t)
	data := randomBytes(12 << 20)
	url := srv.Add("/big.zip", data, httpdltest.ContentType("application/zip"))
	srv.SetDelay(40 * time.Millisecond)

	h := newHarness(t, url)
	ctx, cancel := context.WithCancel(context.Background())
	result := h.start(ctx)
	h.waitForDone(t, 2<<20)
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("paused run returned %v, want context.Canceled", err)
	}
	paused := h.rec.last()
	if paused.Done <= 0 || paused.Done >= int64(len(data)) || !paused.Resumable {
		t.Fatalf("paused at %+v", paused)
	}
	if _, err := os.Stat(filepath.Join(h.job.WorkDir, "state.json")); err != nil {
		t.Fatalf("no resume state: %v", err)
	}

	srv.SetDelay(0)
	srv.ClearRanges()
	h.job.Path = h.rec.claimed()
	if err := h.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	h.checkFile(t, "big.zip", data)
	for _, r := range srv.Ranges() {
		if strings.HasPrefix(r, "bytes=0-") {
			t.Errorf("resume fetched from the start again: %v", srv.Ranges())
			break
		}
	}
}

func TestServerWithoutRanges(t *testing.T) {
	srv := httpdltest.NewServer(t)
	data := randomBytes(3 << 20)
	h := newHarness(t, srv.Add("/norange/file.bin", data, httpdltest.NoRanges()))
	if err := h.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	h.checkFile(t, "file.bin", data)
	if n := len(srv.Ranges()); n != 1 {
		t.Errorf("made %d requests, want 1", n)
	}
	if h.rec.last().Resumable {
		t.Error("reported resumable")
	}
}

func TestUnknownLength(t *testing.T) {
	srv := httpdltest.NewServer(t)
	data := randomBytes(300 << 10)
	h := newHarness(t, srv.Add("/nolength", data, httpdltest.NoRanges(), httpdltest.NoLength()))
	h.job.Filename = "thing.dat"
	if err := h.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	h.checkFile(t, "thing.dat", data)
	if got := h.rec.last(); got.Size != int64(len(data)) || got.Done != got.Size {
		t.Errorf("final progress %+v", got)
	}
}

func TestMissingFile(t *testing.T) {
	srv := httpdltest.NewServer(t)
	h := newHarness(t, srv.URL+"/missing.mp4")
	err := h.run(context.Background())
	var status *StatusError
	if !errors.As(err, &status) || status.Code != 404 || !strings.Contains(err.Error(), "404") {
		t.Fatalf("error = %v, want a 404 StatusError", err)
	}
	if h.rec.claimed() != "" {
		t.Error("claimed a name for a file that isn't there")
	}
}

func TestReconnectsAfterADroppedConnection(t *testing.T) {
	srv := httpdltest.NewServer(t)
	data := randomBytes(3 << 20)
	h := newHarness(t, srv.Add("/flaky.zip", data, httpdltest.DropOnceAfter(1<<20)))
	h.job.Connections = 1
	if err := h.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	h.checkFile(t, "flaky.zip", data)
}

func TestMediaComesInOrderAndCanBeWatched(t *testing.T) {
	srv := httpdltest.NewServer(t)
	data := randomBytes(32 << 20)
	srv.SetDelay(30 * time.Millisecond)
	h := newHarness(t, srv.Add("/watch/clip.mp4", data))

	ctx, cancel := context.WithCancel(context.Background())
	result := h.start(ctx)
	// Well past connections × inOrderChunk, so a file split the usual way would show holes.
	done := h.waitForDone(t, 16<<20)
	path := h.rec.claimed()
	player, err := os.Open(path)
	if err != nil {
		t.Fatalf("can't open the downloading video by its name: %v", err)
	}
	defer player.Close()
	start := make([]byte, inOrderChunk)
	if _, err := io.ReadFull(player, start); err != nil || !bytes.Equal(start, data[:inOrderChunk]) {
		t.Fatalf("its start doesn't play: %v", err)
	}
	cancel()
	<-result //nolint:errcheck // cancelled: the error is expected

	state := h.readState(t)
	firstGap := int64(len(data))
	for _, seg := range state.Segments {
		firstGap = min(firstGap, seg[1])
	}
	if trail := int64(h.job.Connections) * inOrderChunk; firstGap < h.rec.last().Done-trail {
		t.Errorf("the start has a hole at %d with %d done (at %d bytes in)", firstGap, h.rec.last().Done, done)
	}

	srv.SetDelay(0)
	h.job.Path = path
	if err := h.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("finished file is still a symbolic link: %v", err)
	}
	h.checkFile(t, "clip.mp4", data)
	end := make([]byte, 1000)
	if _, err := player.ReadAt(end, int64(len(data)-len(end))); err != nil || !bytes.Equal(end, data[len(data)-len(end):]) {
		t.Errorf("the open player doesn't see the finished file: %v", err)
	}
}

func TestADownloadingFileTellsWhatCanBeRead(t *testing.T) {
	srv := httpdltest.NewServer(t)
	data := randomBytes(16 << 20)
	srv.SetDelay(20 * time.Millisecond)
	h := newHarness(t, srv.Add("/watch/film.mp4", data))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := h.start(ctx)
	h.waitForDone(t, 4<<20)
	file := h.rec.readable()
	if file == nil {
		t.Fatal("the download didn't offer its file for reading")
	}
	n := file.Have(0)
	if n < inOrderChunk {
		t.Fatalf("only %d bytes readable from the start with %d done", n, h.rec.last().Done)
	}
	got := make([]byte, n)
	f, err := os.Open(file.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := io.ReadFull(f, got); err != nil || !bytes.Equal(got, data[:n]) {
		t.Fatalf("the %d bytes said to be there aren't the file's: %v", n, err)
	}
	if p := h.rec.last().Playable; p == 0 || p > h.rec.last().Done {
		t.Errorf("progress says %d playable with %d done", p, h.rec.last().Done)
	}
	cancel()
	<-result //nolint:errcheck // cancelled: the error is expected
}

func TestAPlayerJumpingAheadGetsThereFirst(t *testing.T) {
	srv := httpdltest.NewServer(t)
	data := randomBytes(40 << 20)
	srv.SetDelay(40 * time.Millisecond)
	h := newHarness(t, srv.Add("/watch/long.mp4", data))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := h.start(ctx)
	h.waitForDone(t, 2<<20)
	file := h.rec.readable()
	jump := int64(30<<20 + 4321)
	file.Want(jump)

	deadline := time.Now().Add(10 * time.Second)
	for file.Have(jump) < 256<<10 {
		if time.Now().After(deadline) {
			t.Fatalf("nothing at %d after 10 s, with %d done", jump, h.rec.last().Done)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if done := h.rec.last().Done; done > 20<<20 {
		t.Errorf("the jump came only with %d bytes done: it waited its turn", done)
	}
	got := make([]byte, 256<<10)
	f, err := os.Open(file.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.ReadAt(got, jump); err != nil || !bytes.Equal(got, data[jump:jump+int64(len(got))]) {
		t.Fatalf("the bytes at the jump aren't the file's: %v", err)
	}

	srv.SetDelay(0)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	h.checkFile(t, "long.mp4", data)
}

func TestMP4IndexAtTheEndComesEarly(t *testing.T) {
	// Big enough that, fetched front to back, the index would come long after the first requests.
	media := randomBytes(40 << 20)
	head := slices.Concat(box("ftyp", []byte("isom\x00\x00\x00\x00isommp41")), box("mdat", media))
	indexAt := int64(len(head))
	movie := slices.Concat(head, box("moov", randomBytes(300000)))

	srv := httpdltest.NewServer(t)
	srv.SetDelay(20 * time.Millisecond)
	h := newHarness(t, srv.Add("/backmoov/movie.mp4", movie))
	if err := h.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	h.checkFile(t, "movie.mp4", movie)

	// The index should be requested in the second round; those requests race, so any place in
	// that round counts.
	starts := rangeStarts(srv.Ranges())
	early := starts[1:min(2*h.job.Connections, len(starts))]
	if !slices.ContainsFunc(early, func(s int64) bool { return s >= indexAt-inOrderChunk }) {
		t.Errorf("index at %d not among the first requests: %v", indexAt, early)
	}
}

type harness struct {
	runner    *Runner
	job       engine.Job
	rec       *recorder
	downloads string
}

func newHarness(t *testing.T, url string) *harness {
	t.Helper()
	root := t.TempDir()
	downloads := filepath.Join(root, "downloads")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		t.Fatal(err)
	}
	runner := New()
	runner.backoff = func(int) time.Duration { return 10 * time.Millisecond }
	return &harness{
		runner:    runner,
		job:       engine.Job{URL: url, WorkDir: filepath.Join(root, "work", "1"), Connections: 8},
		rec:       &recorder{dir: downloads},
		downloads: downloads,
	}
}

func (h *harness) run(ctx context.Context) error {
	return h.runner.Run(ctx, h.job, h.rec)
}

func (h *harness) start(ctx context.Context) <-chan error {
	result := make(chan error, 1)
	go func() { result <- h.run(ctx) }()
	return result
}

func (h *harness) waitForDone(t *testing.T, bytes int64) int64 {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if done := h.rec.last().Done; done >= bytes {
			return done
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("only %d bytes done, waited for %d", h.rec.last().Done, bytes)
	return 0
}

func (h *harness) checkFile(t *testing.T, name string, want []byte) {
	t.Helper()
	path := filepath.Join(h.downloads, name)
	if claimed := h.rec.claimed(); claimed != path {
		t.Errorf("named %q, want %q", filepath.Base(claimed), name)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s: got %d bytes that differ from the %d served", name, len(got), len(want))
	}
}

func (h *harness) readState(t *testing.T) resumeState {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(h.job.WorkDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state resumeState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

type recorder struct {
	dir string

	mu       sync.Mutex
	path     string
	progress engine.Progress
	file     engine.Readable
	sum      checksum.Sum
}

func (r *recorder) ClaimPath(name string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.path = naming.UniquePath(r.dir, name, nil)
	return r.path, nil
}

func (r *recorder) Titled(string) {}
func (r *recorder) Note(string)   {}

func (r *recorder) Stage(string) {}
func (r *recorder) Said(string)  {}

func (r *recorder) StageProgress(int64, int64) {}

func (r *recorder) Checksum(sum checksum.Sum) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sum = sum
}

func (r *recorder) Progress(p engine.Progress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.progress = p
}

func (r *recorder) Readable(file engine.Readable) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.file = file
}

func (r *recorder) readable() engine.Readable {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.file
}

func (r *recorder) last() engine.Progress {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.progress
}

func (r *recorder) claimed() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.path
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func box(kind string, payload []byte) []byte {
	b := binary.BigEndian.AppendUint32(nil, uint32(8+len(payload)))
	return append(append(b, kind...), payload...)
}

var rangeStart = regexp.MustCompile(`^bytes=(\d+)-`)

func rangeStarts(ranges []string) []int64 {
	var starts []int64
	for _, r := range ranges {
		if m := rangeStart.FindStringSubmatch(r); m != nil {
			n, _ := strconv.ParseInt(m[1], 10, 64)
			starts = append(starts, n)
		}
	}
	return starts
}

func TestASpeedLimitIsKept(t *testing.T) {
	srv := httpdltest.NewServer(t)
	data := randomBytes(3 << 20)
	h := newHarness(t, srv.Add("/files/limited.bin", data))
	h.job.Limit = ratelimit.New(1 << 20) // 1 MiB/s: about 3 s, less the first burst
	start := time.Now()
	if err := h.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took < 2500*time.Millisecond {
		t.Errorf("3 MiB at 1 MiB/s took only %v", took)
	}
	h.checkFile(t, "limited.bin", data)
}

func TestReportsTheServersChecksum(t *testing.T) {
	srv := httpdltest.NewServer(t)
	data := randomBytes(3 << 20)
	sum := sha256.Sum256(data)
	url := srv.Add("/files/tool.zip", data,
		httpdltest.Header("Repr-Digest", "sha-256=:"+base64.StdEncoding.EncodeToString(sum[:])+":"))

	h := newHarness(t, url)
	if err := h.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	h.rec.mu.Lock()
	defer h.rec.mu.Unlock()
	if want := (checksum.Sum{Algo: checksum.SHA256, Hex: hex.EncodeToString(sum[:])}); h.rec.sum != want {
		t.Fatalf("reported %v, want %v", h.rec.sum, want)
	}
}
