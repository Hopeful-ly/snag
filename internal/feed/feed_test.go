package feed

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func seal(t *testing.T, f Feed, key ed25519.PrivateKey) []byte {
	t.Helper()
	data, err := Seal(f, key)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

var day1 = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

func release(version string) *Release {
	return &Release{Version: version, Page: "https://example.com/v" + version}
}

func TestPublicKeyIsOne(t *testing.T) {
	if _, err := NewChecker("", PublicKey, filepath.Join(t.TempDir(), "feed.json")); err != nil {
		t.Fatal(err)
	}
}

func TestOpen(t *testing.T) {
	pub, priv := newKey(t)
	other, _ := newKey(t)
	good := Feed{Issued: day1, Snag: release("1.2.0"), YtDLP: "nightly@2026.10.20.232845"}

	f, err := Open(seal(t, good, priv), pub)
	if err != nil {
		t.Fatal(err)
	}
	if !f.Issued.Equal(day1) || f.Snag.Version != "1.2.0" || f.YtDLP != good.YtDLP {
		t.Fatalf("read %+v", f)
	}

	if _, err := Open(seal(t, good, priv), other); err == nil {
		t.Fatal("took a feed signed with another key")
	}

	tampered := strings.Replace(string(seal(t, good, priv)), `"feed": "`, `"feed": "e`, 1)
	if _, err := Open([]byte(tampered), pub); err == nil {
		t.Fatal("took a feed changed after signing")
	}

	for name, bad := range map[string]Feed{
		"no date":                    {Snag: release("1.2.0")},
		"yt-dlp option":              {Issued: day1, YtDLP: "--exec=rm"},
		"not a version":              {Issued: day1, Snag: release("1.2")},
		"plain http page":            {Issued: day1, Snag: &Release{Version: "1.2.0", Page: "http://example.com"}},
		"page with a scheme":         {Issued: day1, Snag: &Release{Version: "1.2.0", Page: "file:///etc/passwd"}},
		"plain http installer":       {Issued: day1, Snag: withFile(File{URL: "http://example.com/s.deb", SHA256: sum})},
		"installer without a sum":    {Issued: day1, Snag: withFile(File{URL: "https://example.com/s.deb"})},
		"installer with a short sum": {Issued: day1, Snag: withFile(File{URL: "https://example.com/s.deb", SHA256: sum[:40]})},
	} {
		if _, err := Open(seal(t, bad, priv), pub); err == nil {
			t.Errorf("%s: took it", name)
		}
	}
}

const sum = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

func withFile(f File) *Release {
	r := release("1.2.0")
	r.Files = map[string]File{"deb-amd64": f}
	return r
}

func TestOpenKeepsInstallers(t *testing.T) {
	pub, priv := newKey(t)
	want := File{URL: "https://example.com/snag_1.2.0_amd64.deb", SHA256: sum}
	f, err := Open(seal(t, Feed{Issued: day1, Snag: withFile(want)}, priv), pub)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Snag.Files["deb-amd64"]; got != want {
		t.Fatalf("installer %+v, want %+v", got, want)
	}
}

func TestFileKind(t *testing.T) {
	for name, want := range map[string]string{
		"Snag-1.2.0-windows-amd64-setup.exe": "windows-amd64",
		"Snag-1.2.0-windows-arm64-setup.exe": "windows-arm64",
		"Snag-1.2.0-macos.dmg":               "macos",
		"snag_1.2.0_amd64.deb":               "deb-amd64",
		"snag_1.2.0_arm64.deb":               "deb-arm64",
		"snag-1.2.0-1.x86_64.rpm":            "rpm-amd64",
		"snag-1.2.0-1.aarch64.rpm":           "rpm-arm64",
		"snag-1.2.0-1-x86_64.pkg.tar.zst":    "arch-amd64",
		"snag-1.2.0-1-aarch64.pkg.tar.zst":   "arch-arm64",
		"snag-firefox-1.2.0.xpi":             "",
		"snag-extension-1.2.0.zip":           "",
		"SHA256SUMS":                         "",
	} {
		if got := FileKind(name); got != want {
			t.Errorf("FileKind(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestNewer(t *testing.T) {
	f := Feed{Issued: day1, Snag: release("1.10.0")}
	for running, want := range map[string]bool{
		"1.9.3":  true,
		"0.7.1":  true,
		"1.10.0": false,
		"1.10.1": false,
		"2.0.0":  false,
		"dev":    false,
		"0.0.0":  true,
	} {
		if got := f.Newer(running) != nil; got != want {
			t.Errorf("running %s: newer %v, want %v", running, got, want)
		}
	}
	if (Feed{Issued: day1}).Newer("1.0.0") != nil {
		t.Error("a feed without a release has a newer one")
	}
}

func serve(t *testing.T, body *[]byte) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if *body == nil {
			http.Error(w, "gone", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write(*body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestChecker(t *testing.T) {
	pub, priv := newKey(t)
	key := base64.StdEncoding.EncodeToString(pub)
	path := filepath.Join(t.TempDir(), "feed.json")
	var body []byte
	url := serve(t, &body)
	ctx := context.Background()

	c, err := NewChecker(url, key, path)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Check(ctx); !got.Issued.IsZero() {
		t.Fatalf("with nothing served and nothing kept, got %+v", got)
	}

	body = seal(t, Feed{Issued: day1, Snag: release("1.2.0")}, priv)
	if got := c.Check(ctx); got.Snag == nil || got.Snag.Version != "1.2.0" {
		t.Fatalf("got %+v", got)
	}

	// An older feed, validly signed, is served again: Snag keeps the newer one it has.
	newer := seal(t, Feed{Issued: day1.Add(24 * time.Hour), Snag: release("1.3.0")}, priv)
	body = newer
	c.Check(ctx)
	body = seal(t, Feed{Issued: day1, Snag: release("1.2.0")}, priv)
	if got := c.Check(ctx); got.Snag.Version != "1.3.0" {
		t.Fatalf("an older feed replaced the newer one: %+v", got)
	}

	// Not signed by Snag, or not there: the last good one stays.
	_, stranger := newKey(t)
	body = seal(t, Feed{Issued: day1.Add(48 * time.Hour), Snag: release("9.0.0")}, stranger)
	if got := c.Check(ctx); got.Snag.Version != "1.3.0" {
		t.Fatalf("a feed signed by someone else was taken: %+v", got)
	}
	body = nil
	if got := c.Check(ctx); got.Snag.Version != "1.3.0" {
		t.Fatalf("offline, got %+v", got)
	}

	// Starting again, offline, Snag still knows what it learned.
	again, err := NewChecker(url, key, path)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Known(); got.Snag == nil || got.Snag.Version != "1.3.0" {
		t.Fatalf("after a restart, knew %+v", got)
	}

	// A kept feed is checked again when read, so changing the file on disk doesn't help either.
	otherPub, _ := newKey(t)
	signedByOther, err := NewChecker(url, base64.StdEncoding.EncodeToString(otherPub), path)
	if err != nil {
		t.Fatal(err)
	}
	if got := signedByOther.Known(); !got.Issued.IsZero() {
		t.Fatalf("a kept feed wasn't checked again: %+v", got)
	}
}

func TestTellOncePerVersion(t *testing.T) {
	pub, _ := newKey(t)
	path := filepath.Join(t.TempDir(), "feed.json")
	c, err := NewChecker("", base64.StdEncoding.EncodeToString(pub), path)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Tell(release("1.2.0")) || c.Tell(release("1.2.0")) {
		t.Fatal("not told exactly once about 1.2.0")
	}
	again, _ := NewChecker("", base64.StdEncoding.EncodeToString(pub), path)
	if again.Tell(release("1.2.0")) {
		t.Fatal("told again about 1.2.0 after a restart")
	}
	if !again.Tell(release("1.3.0")) {
		t.Fatal("not told about 1.3.0")
	}
	if c.Tell(nil) {
		t.Fatal("told about no release")
	}
}

func TestTooLarge(t *testing.T) {
	pub, _ := newKey(t)
	body := []byte(strings.Repeat(" ", maxSize+1))
	c, _ := NewChecker(serve(t, &body), base64.StdEncoding.EncodeToString(pub), filepath.Join(t.TempDir(), "feed.json"))
	if _, err := c.fetch(context.Background()); err == nil {
		t.Fatal("read a feed larger than any real one")
	}
}
