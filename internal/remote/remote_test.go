package remote

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeDownloads struct {
	mu    sync.Mutex
	items []Item
	calls []string
}

func (f *fakeDownloads) record(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
}

func (f *fakeDownloads) Look(link string) (Look, error) {
	f.record("look " + link)
	return Look{Title: "A talk", Choices: []Option{{Label: "1080p"}, {Label: "Audio MP3"}}}, nil
}

func (f *fakeDownloads) Add(link string, choice int) (Item, error) {
	if choice < 0 {
		f.record("add " + link)
	} else {
		f.record(fmt.Sprint("add ", link, " at ", choice))
	}
	item := Item{ID: fmt.Sprint("d", len(f.items)+1), Name: "file.mp4", Status: "downloading"}
	f.mu.Lock()
	f.items = append(f.items, item)
	f.mu.Unlock()
	return item, nil
}

func (f *fakeDownloads) List() []Item {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Item(nil), f.items...)
}

func (f *fakeDownloads) Pause(id string) error  { f.record("pause " + id); return nil }
func (f *fakeDownloads) Resume(id string) error { f.record("resume " + id); return nil }
func (f *fakeDownloads) Forget(id string) error {
	f.record("forget " + id)
	if id == "missing" {
		return errors.New("no such download")
	}
	return nil
}

func newRemote(t *testing.T) (*Remote, *fakeDownloads, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "remote.json")
	d := &fakeDownloads{}
	return New(d, path), d, path
}

func TestPairing(t *testing.T) {
	r, _, path := newRemote(t)
	if _, _, err := r.Pair("anything", "Phone"); !errors.Is(err, ErrNotPairing) {
		t.Fatalf("paired with pairing closed: %v", err)
	}

	code, _ := r.StartPairing()
	token, d, err := r.Pair(code, "iPhone,\nSafari\x00")
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "iPhone, Safari" {
		t.Errorf("named %q", d.Name)
	}
	if _, _, err := r.Pair(code, "Again"); !errors.Is(err, ErrNotPairing) {
		t.Fatalf("a code worked twice: %v", err)
	}
	if got, err := r.Device(token); err != nil || got.ID != d.ID {
		t.Fatalf("the token isn't %s's: %v", d.ID, err)
	}

	// Only the token's hash is kept.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(token)) || !bytes.Contains(data, []byte(hashOf(token))) {
		t.Fatal("the token itself was kept")
	}

	// After a restart the phone is still paired; removed, it's locked out at once.
	again := New(&fakeDownloads{}, path)
	if _, err := again.Device(token); err != nil {
		t.Fatalf("after a restart: %v", err)
	}
	again.RemoveDevice(d.ID)
	if _, err := again.Device(token); !errors.Is(err, ErrUnknownDevice) {
		t.Fatalf("a removed phone still works: %v", err)
	}
	if len(again.Devices()) != 0 {
		t.Fatal("the removed phone is still listed")
	}
}

func TestPairingExpiresAndCloses(t *testing.T) {
	r, _, _ := newRemote(t)
	now := time.Now()
	r.now = func() time.Time { return now }

	code, until := r.StartPairing()
	if until.Sub(now) != pairFor {
		t.Fatalf("open for %v", until.Sub(now))
	}
	now = now.Add(pairFor + time.Second)
	if _, _, err := r.Pair(code, "Late"); !errors.Is(err, ErrNotPairing) {
		t.Fatalf("an expired code worked: %v", err)
	}

	code, _ = r.StartPairing()
	for i := range pairTries {
		if _, _, err := r.Pair("wrong", "Guess"); !errors.Is(err, ErrWrongCode) {
			t.Fatalf("try %d: %v", i, err)
		}
	}
	if _, _, err := r.Pair(code, "Right, too late"); !errors.Is(err, ErrNotPairing) {
		t.Fatalf("pairing stayed open after %d wrong tries: %v", pairTries, err)
	}

	code, _ = r.StartPairing()
	r.StopPairing()
	if _, _, err := r.Pair(code, "Stopped"); !errors.Is(err, ErrNotPairing) {
		t.Fatalf("paired after stopping: %v", err)
	}
}

// A paired phone's invite pairs another copy of the page, once, without closing the QR code
// open in Settings; wrong tries close both.
func TestInvite(t *testing.T) {
	r, _, _ := newRemote(t)
	now := time.Now()
	r.now = func() time.Time { return now }

	qr, _ := r.StartPairing()
	resp := r.Handle(Request{Op: "invite"})
	if resp.Code == "" || resp.Code == qr {
		t.Fatalf("invite gave %q", resp.Code)
	}
	if _, d, err := r.Pair(resp.Code, "iPhone, home screen"); err != nil || d.Name != "iPhone, home screen" {
		t.Fatalf("the invite didn't pair: %v", err)
	}
	if _, _, err := r.Pair(resp.Code, "Again"); !errors.Is(err, ErrWrongCode) {
		t.Fatalf("an invite worked twice: %v", err)
	}
	if _, _, err := r.Pair(qr, "From the QR code"); err != nil {
		t.Fatalf("the invite closed the QR code: %v", err)
	}
	if n := len(r.Devices()); n != 2 {
		t.Fatalf("%d devices", n)
	}

	invite := r.Invite()
	now = now.Add(pairFor + time.Second)
	if _, _, err := r.Pair(invite, "Late"); !errors.Is(err, ErrNotPairing) {
		t.Fatalf("an expired invite worked: %v", err)
	}

	qr, _ = r.StartPairing()
	invite = r.Invite()
	for range pairTries {
		_, _, _ = r.Pair("wrong", "Guess")
	}
	for _, code := range []string{qr, invite} {
		if _, _, err := r.Pair(code, "Too late"); !errors.Is(err, ErrNotPairing) {
			t.Fatalf("pairing stayed open after %d wrong tries: %v", pairTries, err)
		}
	}
}

func TestHandle(t *testing.T) {
	r, d, _ := newRemote(t)
	if res := r.Handle(Request{Op: "add", Link: "https://example.com/v.mp4"}); res.Error != "" || res.Item == nil {
		t.Fatalf("add: %+v", res)
	}
	for _, bad := range []string{"", "file:///etc/passwd", "javascript:alert(1)", "https://", "ftp://example.com/x"} {
		if res := r.Handle(Request{Op: "add", Link: bad}); res.Error == "" {
			t.Errorf("added %q", bad)
		}
	}
	if res := r.Handle(Request{Op: "look", Link: "https://example.com/talk"}); res.Look == nil || len(res.Look.Choices) != 2 {
		t.Fatalf("look: %+v", res)
	}
	if res := r.Handle(Request{Op: "look", Link: "file:///etc/passwd"}); res.Error == "" {
		t.Error("looked at a file:// link")
	}
	one, minusOne := 1, -1
	if res := r.Handle(Request{Op: "add", Link: "https://example.com/talk", Choice: &one}); res.Error != "" {
		t.Fatalf("add at a quality: %+v", res)
	}
	if res := r.Handle(Request{Op: "add", Link: "https://example.com/talk", Choice: &minusOne}); res.Error == "" {
		t.Error("added at quality -1")
	}
	if res := r.Handle(Request{Op: "list"}); len(res.Items) != 2 {
		t.Fatalf("list: %+v", res)
	}
	for _, op := range []string{"pause", "resume", "forget"} {
		if res := r.Handle(Request{Op: op, ID: "d1"}); res.Error != "" {
			t.Errorf("%s: %s", op, res.Error)
		}
		if res := r.Handle(Request{Op: op}); res.Error == "" {
			t.Errorf("%s without a download worked", op)
		}
	}
	if res := r.Handle(Request{Op: "forget", ID: "missing"}); res.Error != "no such download" {
		t.Errorf("forget's error: %q", res.Error)
	}
	for _, op := range []string{"delete", "settings", ""} {
		if res := r.Handle(Request{Op: op, ID: "d1"}); res.Error == "" {
			t.Errorf("%q worked", op)
		}
	}
	want := "add https://example.com/v.mp4,look https://example.com/talk,add https://example.com/talk at 1," +
		"pause d1,resume d1,forget d1,forget missing"
	if got := strings.Join(d.calls, ","); got != want {
		t.Errorf("calls %s, want %s", got, want)
	}
}
