package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/checksum"
	"github.com/Roham1299/snag/internal/cookies"
)

// The fake runner's files say "done".
var doneSum = func() string {
	sum := sha256.Sum256([]byte("done"))
	return hex.EncodeToString(sum[:])
}()

func (te *testEngine) waitCheck(t *testing.T, id string, results ...CheckResult) Download {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		d, err := te.Get(id)
		if err == nil {
			for _, r := range results {
				if d.Check.Result == r {
					return d
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("download %s's check is %+v (err %v), want one of %v", id, d.Check, err, results)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (te *testEngine) finished(t *testing.T, req AddRequest) Download {
	t.Helper()
	d := te.add(t, req)
	te.waitFor(t, d.ID, StatusDownloading)
	te.runner.finish(d.URL, nil)
	return te.waitFor(t, d.ID, StatusDone)
}

func TestAPagesChecksumIsCheckedOnlyWhenAsked(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.finished(t, AddRequest{URL: "https://example.com/tool.zip", Checksum: "SHA-256: " + strings.ToUpper(doneSum)})
	if c := d.Check; c.Want != doneSum || c.Algo != checksum.SHA256 || c.From != FromPage || c.Result != CheckNone {
		t.Fatalf("check before asking = %+v", c)
	}
	time.Sleep(50 * time.Millisecond)
	if d, _ := te.Get(d.ID); d.Check.Result != CheckNone {
		t.Fatalf("checked without asking: %+v", d.Check)
	}

	if err := te.Check(d.ID, ""); err != nil {
		t.Fatal(err)
	}
	d = te.waitCheck(t, d.ID, CheckMatch)
	if d.Check.Got != doneSum || d.Status != StatusDone {
		t.Fatalf("after checking: %+v", d)
	}

	// The result survives a restart.
	te.Shutdown(context.Background())
	te.start(t)
	if d, _ := te.Get(d.ID); d.Check.Result != CheckMatch {
		t.Fatalf("after a restart: %+v", d.Check)
	}
}

func TestAFileThatDoesntMatch(t *testing.T) {
	te := newTestEngine(t, Settings{})
	other := strings.Repeat("ab", 32)
	d := te.finished(t, AddRequest{URL: "https://example.com/tool.zip", Checksum: other})
	if err := te.Check(d.ID, ""); err != nil {
		t.Fatal(err)
	}
	d = te.waitCheck(t, d.ID, CheckMismatch)
	if d.Check.Got != doneSum || d.Check.Want != other {
		t.Fatalf("after checking: %+v", d.Check)
	}
}

func TestAPastedChecksumWins(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.finished(t, AddRequest{URL: "https://example.com/tool.zip", Checksum: strings.Repeat("ab", 32)})
	if err := te.Check(d.ID, doneSum+"  tool.zip"); err != nil {
		t.Fatal(err)
	}
	d = te.waitCheck(t, d.ID, CheckMatch)
	if d.Check.From != FromYou {
		t.Fatalf("from %q, want the user", d.Check.From)
	}
}

func TestCheckingWhatCantBeChecked(t *testing.T) {
	te := newTestEngine(t, Settings{})
	running := te.add(t, AddRequest{URL: "https://example.com/running.zip"})
	te.waitFor(t, running.ID, StatusDownloading)
	if err := te.Check(running.ID, doneSum); !errors.Is(err, ErrNotFinished) {
		t.Errorf("checking a running download: %v", err)
	}
	if err := te.Check("nope", ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("checking nothing: %v", err)
	}

	d := te.finished(t, AddRequest{URL: "https://example.com/plain.zip"})
	if err := te.Check(d.ID, ""); !errors.Is(err, ErrNoChecksum) {
		t.Errorf("checking without a checksum: %v", err)
	}
	if err := te.Check(d.ID, "not a checksum"); !errors.Is(err, ErrBadChecksum) {
		t.Errorf("checking against nonsense: %v", err)
	}
	if err := os.Remove(d.Path); err != nil {
		t.Fatal(err)
	}
	if err := te.Check(d.ID, doneSum); err == nil {
		t.Error("checked a file that is gone")
	}
}

func TestChecksumsAreLookedForOnceAFileIsDone(t *testing.T) {
	var mu sync.Mutex
	var asked []Download
	te := newTestEngineWith(t, Settings{}, func(ctx context.Context, d Download) (checksum.Sum, string, error) {
		mu.Lock()
		defer mu.Unlock()
		asked = append(asked, d)
		if strings.HasSuffix(d.URL, "/none.zip") {
			return checksum.Sum{}, "", nil
		}
		if strings.HasSuffix(d.URL, "/broken.zip") {
			return checksum.Sum{}, "", errors.New("offline")
		}
		return checksum.Sum{Algo: checksum.SHA256, Hex: doneSum}, FromGitHub, nil
	})

	jar := []cookies.Cookie{{Name: "session", Value: "1", Domain: "example.com"}}
	d := te.finished(t, AddRequest{URL: "https://example.com/tool.zip", Cookies: jar})
	d = te.waitCheck(t, d.ID, CheckNone)
	if d.Check.Want != doneSum || d.Check.From != FromGitHub {
		t.Fatalf("after looking: %+v", d.Check)
	}
	for _, url := range []string{"https://example.com/none.zip", "https://example.com/broken.zip"} {
		other := te.finished(t, AddRequest{URL: url})
		if other = te.waitCheck(t, other.ID, CheckNone); other.Check.Found() {
			t.Errorf("%s: found %+v", url, other.Check)
		}
	}
	// A checksum known already isn't looked for, nor is a stream's.
	te.finished(t, AddRequest{URL: "https://example.com/known.zip", Checksum: doneSum})
	te.finished(t, AddRequest{URL: "https://example.com/video.m3u8"})
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(asked) != 3 {
		t.Fatalf("looked %d times, want 3", len(asked))
	}
	if first := asked[0]; len(first.Cookies) != 1 || first.Path == "" || first.Status != StatusDone {
		t.Errorf("the finder was given %+v", first)
	}
}

func TestLoadForgetsChecksCutShort(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.finished(t, AddRequest{URL: "https://example.com/tool.zip", Checksum: doneSum})
	te.Shutdown(context.Background())

	var list []Download
	if err := te.store.Load(&list); err != nil {
		t.Fatal(err)
	}
	list[0].Check.Result, list[0].Check.Read = CheckChecking, 12
	if err := te.store.Save(list); err != nil {
		t.Fatal(err)
	}
	te.start(t)
	if got, _ := te.Get(d.ID); got.Check.Result != CheckNone || got.Check.Read != 0 || got.Check.Want != doneSum {
		t.Fatalf("after a restart: %+v", got.Check)
	}
}

func TestRedownloadReplacesTheFile(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.finished(t, AddRequest{URL: "https://example.com/tool.zip", Checksum: strings.Repeat("ab", 32)})
	if err := os.WriteFile(d.Path, []byte("damaged"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := te.Redownload("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("redownloading nothing: %v", err)
	}

	fresh, err := te.Redownload(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := te.Get(d.ID); !errors.Is(err, ErrNotFound) {
		t.Error("the old download is still listed")
	}
	if fresh.Check.Want != d.Check.Want || fresh.Check.From != FromPage || fresh.Status == StatusPending {
		t.Errorf("the new download = %+v", fresh)
	}
	te.waitFor(t, fresh.ID, StatusDownloading)
	te.runner.finish(fresh.URL, nil)
	fresh = te.waitFor(t, fresh.ID, StatusDone)
	if fresh.Path != d.Path {
		t.Errorf("saved as %s, want %s", fresh.Path, d.Path)
	}
	if data, _ := os.ReadFile(d.Path); string(data) != "done" {
		t.Errorf("the file says %q", data)
	}
	if entries, _ := os.ReadDir(filepath.Dir(d.Path)); len(entries) != 1 {
		t.Errorf("the folder has %d files, want 1", len(entries))
	}
}
