package remote

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"testing/fstest"
)

// testKeeper fakes the computer's network and addresses; tests call refresh themselves.
func testKeeper(t *testing.T, network string, addrs *[]netip.Addr) (*Keeper, *Remote) {
	t.Helper()
	r, _, _ := newRemote(t)
	k := NewKeeper(r, nil, slog.New(slog.DiscardHandler))
	k.current = func() (string, error) { return network, nil }
	k.addrs = func(networks []string) ([]netip.Addr, error) {
		if len(networks) != 1 || networks[0] != network {
			return nil, nil
		}
		return *addrs, nil
	}
	t.Cleanup(k.Close)
	return k, r
}

func TestKeeper(t *testing.T) {
	addrs := []netip.Addr{netip.MustParseAddr("127.0.0.1")}
	k, r := testKeeper(t, "127.0.0.0/8", &addrs)
	changes := make(chan struct{}, 10)
	k.OnChange(func() { changes <- struct{}{} })

	if k.On() || k.Addresses() != nil {
		t.Fatal("on before being turned on")
	}
	if err := k.TurnOn(); err != nil {
		t.Fatal(err)
	}
	<-changes
	if !k.On() || len(k.Addresses()) != 1 || !strings.HasPrefix(k.Addresses()[0], "http://127.0.0.1:") {
		t.Fatalf("turned on: on %v, addresses %v", k.On(), k.Addresses())
	}
	first := k.Addresses()[0]

	// Nothing changed: the same server.
	k.refresh()
	if k.Addresses()[0] != first {
		t.Fatal("moved without a reason")
	}

	// The computer left the network: no server; back on it, the same port as before.
	addrs = nil
	k.refresh()
	<-changes
	if k.Addresses() != nil {
		t.Fatalf("still listening off the network: %v", k.Addresses())
	}
	addrs = []netip.Addr{netip.MustParseAddr("127.0.0.1")}
	k.refresh()
	<-changes
	if len(k.Addresses()) != 1 || k.Addresses()[0] != first {
		t.Fatalf("back on the network at %v, not %s", k.Addresses(), first)
	}

	// Turned off: no server, pairing closed, and it stays off when Snag starts again.
	r.StartPairing()
	k.TurnOff()
	if k.On() || k.Addresses() != nil {
		t.Fatal("still on after turning off")
	}
	if _, _, err := r.Pair("anything", ""); !errors.Is(err, ErrNotPairing) {
		t.Fatalf("pairing still open after turning off: %v", err)
	}
	k.Start()
	if k.Addresses() != nil {
		t.Fatal("started though turned off")
	}
}

func TestKeeperOnAnotherNetwork(t *testing.T) {
	var addrs []netip.Addr // not on the network: no address in it
	k, _ := testKeeper(t, "192.168.1.0/24", &addrs)
	if err := k.TurnOn(); !errors.Is(err, ErrNoNetwork) {
		t.Fatalf("turning on without an address: %v", err)
	}
	if !k.On() {
		t.Fatal("not left on, for when the computer is back on the network")
	}
}

func TestPage(t *testing.T) {
	build := fstest.MapFS{
		"remote.html":         {Data: []byte("<p>phone</p>")},
		"index.html":          {Data: []byte("<p>window</p>")},
		"assets/remote-1.js":  {Data: []byte("go()")},
		"assets/remote-1.css": {Data: []byte("p{}")},
	}
	page := Page(build, []byte("png"))
	r, _, _ := newRemote(t)
	_, c := serveLocal(t, r, page)

	get := func(path string) (int, string, http.Header) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, c.base+path, nil)
		req.Host = c.host
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body), resp.Header
	}

	status, body, h := get("/")
	if status != http.StatusOK || body != "<p>phone</p>" {
		t.Fatalf("/: %d %q", status, body)
	}
	if !strings.Contains(h.Get("Content-Security-Policy"), "default-src 'self'") ||
		!strings.Contains(h.Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatalf("policy %q", h.Get("Content-Security-Policy"))
	}
	if status, body, _ := get("/assets/remote-1.js"); status != http.StatusOK || body != "go()" {
		t.Fatalf("script: %d %q", status, body)
	}
	if status, body, h := get("/icon.png"); status != http.StatusOK || body != "png" || h.Get("Content-Type") != "image/png" {
		t.Fatalf("icon: %d %q", status, body)
	}
	for _, path := range []string{"/index.html", "/remote.html", "/assets/", "/assets/../index.html", "/assets/missing.js", "/pair/x"} {
		if status, _, _ := get(path); status != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", path, status)
		}
	}
}
