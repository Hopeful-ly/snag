package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/Roham1299/snag/internal/checksum"
	"github.com/Roham1299/snag/internal/cookies"
	"github.com/Roham1299/snag/internal/engine"
)

var (
	sumA = strings.Repeat("a1", 32)
	sumB = strings.Repeat("b2", 32)
)

// sumsServer serves files by path and records the requests.
type sumsServer struct {
	*httptest.Server
	mu    sync.Mutex
	asked []string
	files map[string]string
	types map[string]string
	seen  http.Header
}

func newSumsServer(t *testing.T, files map[string]string) *sumsServer {
	s := &sumsServer{files: files, types: map[string]string{}}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.asked = append(s.asked, r.URL.Path)
		s.seen = r.Header.Clone()
		body, ok := s.files[r.URL.Path]
		kind := s.types[r.URL.Path]
		s.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		if kind == "" {
			kind = "text/plain"
		}
		w.Header().Set("Content-Type", kind)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *sumsServer) requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.asked...)
}

func finderFor(t *testing.T) *checksumFinder {
	t.Helper()
	f := newChecksumFinder()
	f.gitHubHost = "github.invalid"
	return f
}

func TestFindsChecksumsNextToAFile(t *testing.T) {
	srv := newSumsServer(t, map[string]string{
		"/releases/SHA256SUMS": sumB + "  other.iso\n" + sumA + " *distro-1.0.iso\n",
	})
	d := engine.Download{URL: srv.URL + "/releases/distro-1.0.iso", Path: "/home/me/Downloads/distro-1.0 (1).iso"}
	sum, from, err := finderFor(t).Find(context.Background(), d)
	if err != nil || from != engine.FromFile || sum != (checksum.Sum{Algo: checksum.SHA256, Hex: sumA}) {
		t.Fatalf("Find = %v, %q, %v", sum, from, err)
	}
	if got := srv.requests(); len(got) != 4 || got[0] != "/releases/distro-1.0.iso.sha256" {
		t.Errorf("asked for %v", got)
	}
}

func TestFindsAChecksumFileForOneFile(t *testing.T) {
	srv := newSumsServer(t, map[string]string{"/dl/tool.zip.sha256": sumA + "\n"})
	sum, _, err := finderFor(t).Find(context.Background(), engine.Download{URL: srv.URL + "/dl/tool.zip?token=1"})
	if err != nil || sum.Hex != sumA {
		t.Fatalf("Find = %v, %v", sum, err)
	}
}

func TestOnlyProgramsAndImagesAreLookedForBesideThemselves(t *testing.T) {
	srv := newSumsServer(t, map[string]string{"/v/SHA256SUMS": sumA + "  film.mp4\n"})
	sum, _, err := finderFor(t).Find(context.Background(), engine.Download{URL: srv.URL + "/v/film.mp4"})
	if err != nil || !sum.IsZero() || len(srv.requests()) != 0 {
		t.Fatalf("Find = %v, %v; asked %v", sum, err, srv.requests())
	}
}

func TestThePagesChecksumFilesComeFirst(t *testing.T) {
	srv := newSumsServer(t, map[string]string{
		"/notes.html":   "<html>" + sumB + "  tool.zip</html>",
		"/CHECKSUMS":    "SHA512 (other) = " + strings.Repeat("c3", 64) + "\nSHA256 (tool.zip) = " + sumA + "\n",
		"/tool.zip.md5": strings.Repeat("d4", 16) + "\n",
	})
	srv.types["/notes.html"] = "text/html"
	d := engine.Download{
		URL:           srv.URL + "/files/tool.zip",
		ChecksumFiles: []string{srv.URL + "/notes.html", srv.URL + "/missing", srv.URL + "/CHECKSUMS"},
		Headers:       map[string]string{"Referer": srv.URL + "/download"},
		Cookies:       []cookies.Cookie{{Name: "s", Value: "1", Domain: "127.0.0.1"}},
	}
	sum, from, err := finderFor(t).Find(context.Background(), d)
	if err != nil || from != engine.FromFile || sum.Hex != sumA {
		t.Fatalf("Find = %v, %q, %v", sum, from, err)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.seen.Get("Cookie") != "s=1" || srv.seen.Get("Referer") == "" {
		t.Errorf("asked without the download's cookies or referrer: %v", srv.seen)
	}
}

func TestNothingFoundIsNoFailure(t *testing.T) {
	srv := newSumsServer(t, map[string]string{})
	sum, _, err := finderFor(t).Find(context.Background(), engine.Download{URL: srv.URL + "/a/setup.exe"})
	if err != nil || !sum.IsZero() {
		t.Fatalf("Find = %v, %v", sum, err)
	}
}

func TestFindsAGitHubReleasesChecksum(t *testing.T) {
	api := newSumsServer(t, map[string]string{
		"/repos/owner/tool/releases/tags/v1.2.0": `{"assets":[
			{"name":"tool-linux.tar.gz","digest":"sha256:` + sumB + `"},
			{"name":"tool setup.exe","digest":"sha256:` + sumA + `"}]}`,
		"/repos/owner/tool/releases/latest": `{"assets":[{"name":"tool-linux.tar.gz","digest":"sha256:` + sumB + `"}]}`,
	})
	f := newChecksumFinder()
	f.gitHubAPI = api.URL
	f.gitHubHost = "github.com"
	f.client = &http.Client{Transport: onlyTo(api.URL)}

	for _, tc := range []struct{ address, want string }{
		{"https://github.com/owner/tool/releases/download/v1.2.0/tool%20setup.exe", sumA},
		{"https://github.com/owner/tool/releases/latest/download/tool-linux.tar.gz", sumB},
	} {
		sum, from, err := f.Find(context.Background(), engine.Download{URL: tc.address})
		if err != nil || from != engine.FromGitHub || sum.Hex != tc.want {
			t.Errorf("Find(%s) = %v, %q, %v", tc.address, sum, from, err)
		}
	}
}

// onlyTo redirects every request to base, so github.com addresses reach the fake API and none
// leave the machine.
func onlyTo(base string) http.RoundTripper {
	return roundTrip(func(r *http.Request) (*http.Response, error) {
		target, _ := url.Parse(base)
		r = r.Clone(r.Context())
		r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
		return http.DefaultTransport.RoundTrip(r)
	})
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
