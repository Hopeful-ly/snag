package remote

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
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

func (f *fakeDownloads) Add(link string) (Item, error) {
	f.record("add " + link)
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
	if res := r.Handle(Request{Op: "list"}); len(res.Items) != 1 {
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
	want := "add https://example.com/v.mp4,pause d1,resume d1,forget d1,forget missing"
	if got := strings.Join(d.calls, ","); got != want {
		t.Errorf("calls %s, want %s", got, want)
	}
}

func cidr(t *testing.T, s string) net.Addr {
	t.Helper()
	ip, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatal(err)
	}
	n.IP = ip
	return n
}

func TestListenAddrs(t *testing.T) {
	ifaces := []net.Addr{
		cidr(t, "127.0.0.1/8"),
		cidr(t, "192.168.1.5/24"),
		cidr(t, "10.8.0.2/24"),
		cidr(t, "203.0.113.7/24"), // public
		cidr(t, "fd12:3456::5/64"),
		cidr(t, "fe80::1/64"), // link-local
	}
	got := listenAddrs(ifaces, []string{"192.168.1.0/24", "203.0.113.0/24", "fd12:3456::/64", "127.0.0.0/8"})
	want := []netip.Addr{netip.MustParseAddr("192.168.1.5"), netip.MustParseAddr("fd12:3456::5")}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("listens on %v, want %v: never public, loopback, or outside the networks", got, want)
	}
	if got := listenAddrs(ifaces, nil); len(got) != 0 {
		t.Fatalf("with no networks, listens on %v", got)
	}
}

func TestNetworkOf(t *testing.T) {
	ifaces := []net.Addr{cidr(t, "172.17.0.1/16"), cidr(t, "192.168.91.26/22"), cidr(t, "203.0.113.7/24")}
	if got, ok := networkOf(ifaces, netip.MustParseAddr("192.168.91.26")); !ok || got != "192.168.88.0/22" {
		t.Fatalf("the home network is %q", got)
	}
	if got, ok := networkOf(ifaces, netip.MustParseAddr("203.0.113.7")); ok {
		t.Fatalf("a public network was taken for home: %q", got)
	}
}

type client struct {
	t    *testing.T
	base string
	host string
}

func (c client) post(path, token, contentType string, body any) (int, map[string]any) {
	c.t.Helper()
	data, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, c.base+path, bytes.NewReader(data))
	if err != nil {
		c.t.Fatal(err)
	}
	req.Host = c.host
	req.Header.Set("Content-Type", contentType)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func serveLocal(t *testing.T, r *Remote, page http.Handler) (*Server, client) {
	t.Helper()
	s, err := start(r, page, slog.New(slog.DiscardHandler), []netip.Addr{netip.MustParseAddr("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	base := s.Addresses()[0]
	return s, client{t: t, base: base, host: strings.TrimPrefix(base, "http://")}
}

func TestServer(t *testing.T) {
	r, d, _ := newRemote(t)
	page := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "the page") })
	_, c := serveLocal(t, r, page)

	code, _ := r.StartPairing()
	status, paired := c.post("/pair", "", "application/json", pairRequest{Code: code, Name: "Pixel, Chrome"})
	if status != http.StatusOK {
		t.Fatalf("pairing: %d %v", status, paired)
	}
	token := paired["token"].(string)

	status, res := c.post("/api", token, "application/json", Request{Op: "add", Link: "https://example.com/a.mp4"})
	if status != http.StatusOK || res["item"] == nil {
		t.Fatalf("add: %d %v", status, res)
	}
	if status, res := c.post("/api", token, "application/json", Request{Op: "list"}); status != http.StatusOK || len(res["items"].([]any)) != 1 {
		t.Fatalf("list: %d %v", status, res)
	}

	// Refused: no token, a wrong one, the wrong type, unknown fields, a code tried again.
	if status, _ := c.post("/api", "", "application/json", Request{Op: "list"}); status != http.StatusUnauthorized {
		t.Errorf("no token: %d", status)
	}
	if status, _ := c.post("/api", "wrong", "application/json", Request{Op: "list"}); status != http.StatusUnauthorized {
		t.Errorf("wrong token: %d", status)
	}
	if status, _ := c.post("/api", token, "text/plain", Request{Op: "list"}); status != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain, which a page elsewhere can send without asking: %d", status)
	}
	if status, _ := c.post("/api", token, "application/json", map[string]string{"op": "list", "path": "/"}); status != http.StatusBadRequest {
		t.Errorf("unknown fields: %d", status)
	}
	if status, _ := c.post("/pair", "", "application/json", pairRequest{Code: code}); status != http.StatusForbidden {
		t.Errorf("a used code: %d", status)
	}

	// The removed phone is locked out.
	r.RemoveDevice(paired["device"].(map[string]any)["id"].(string))
	if status, _ := c.post("/api", token, "application/json", Request{Op: "list"}); status != http.StatusUnauthorized {
		t.Errorf("a removed phone: %d", status)
	}

	// Everything else is the page; the API without a token is refused before anything else.
	resp, err := http.Get(c.base + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "the page" || resp.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("the page: %q, CORS %q", body, resp.Header.Get("Access-Control-Allow-Origin"))
	}
	resp, err = http.Get(c.base + "/api")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("GET /api: %d", resp.StatusCode)
	}
	if got := strings.Join(d.calls, ","); got != "add https://example.com/a.mp4" {
		t.Errorf("calls: %s", got)
	}
}

func TestServerWrongHost(t *testing.T) {
	r, _, _ := newRemote(t)
	_, c := serveLocal(t, r, nil)
	c.host = "snag.attacker.example:80" // DNS rebinding: the name points at this computer
	code, _ := r.StartPairing()
	if status, _ := c.post("/pair", "", "application/json", pairRequest{Code: code}); status != http.StatusMisdirectedRequest {
		t.Fatalf("another Host: %d", status)
	}
}

func TestServerRateLimit(t *testing.T) {
	r, _, _ := newRemote(t)
	_, c := serveLocal(t, r, nil)
	for i := range perMinute {
		if status, _ := c.post("/api", "wrong", "application/json", Request{Op: "list"}); status != http.StatusUnauthorized {
			t.Fatalf("request %d: %d", i, status)
		}
	}
	if status, _ := c.post("/api", "wrong", "application/json", Request{Op: "list"}); status != http.StatusTooManyRequests {
		t.Fatalf("past the limit: %d", status)
	}
}

func TestServerKeepsItsPort(t *testing.T) {
	r, _, path := newRemote(t)
	s, _ := serveLocal(t, r, nil)
	first := s.Addresses()[0]
	_ = s.Close()

	again, _ := serveLocal(t, New(&fakeDownloads{}, path), nil)
	if got := again.Addresses()[0]; got != first {
		t.Fatalf("listened on %s, then %s", first, got)
	}
}
