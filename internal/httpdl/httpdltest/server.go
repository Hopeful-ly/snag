// Package httpdltest serves files for download tests.
package httpdltest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"
)

const chunkSize = 64 << 10

// Server is a test file server.
type Server struct {
	*httptest.Server

	mu     sync.Mutex
	files  map[string]*file
	ranges []string
	delay  time.Duration
}

type file struct {
	body        []byte
	contentType string
	noRanges    bool
	noLength    bool
	filename    string
	dropAfter   int64
	headers     http.Header
}

// Option changes how one file is served.
type Option func(*file)

// NoRanges serves the file whole, ignoring Range headers.
func NoRanges() Option { return func(f *file) { f.noRanges = true } }

// NoLength leaves out Content-Length, so the size is only known at the end.
func NoLength() Option { return func(f *file) { f.noLength = true } }

// Filename suggests a name in a Content-Disposition header.
func Filename(name string) Option { return func(f *file) { f.filename = name } }

// ContentType sets the Content-Type header (video/mp4 by default).
func ContentType(t string) Option { return func(f *file) { f.contentType = t } }

// Header adds a header to every response for the file.
func Header(name, value string) Option {
	return func(f *file) {
		if f.headers == nil {
			f.headers = http.Header{}
		}
		f.headers.Add(name, value)
	}
}

// DropOnceAfter cuts the first response for the file off after n bytes.
func DropOnceAfter(n int64) Option { return func(f *file) { f.dropAfter = n } }

// NewServer starts a server that stops when the test ends.
func NewServer(t testing.TB) *Server {
	s := &Server{files: map[string]*file{}}
	s.Server = httptest.NewServer(s)
	t.Cleanup(s.Close)
	return s
}

// Add serves body at path and returns its address.
func (s *Server) Add(path string, body []byte, opts ...Option) string {
	f := &file{body: body, contentType: "video/mp4"}
	for _, opt := range opts {
		opt(f)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[path] = f
	return s.URL + path
}

// SetDelay makes the server pause this long after every 64 KiB it sends.
func (s *Server) SetDelay(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delay = d
}

// Ranges returns the Range header of every request so far, in order ("" for none).
func (s *Server) Ranges() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.ranges)
}

// ClearRanges forgets the requests seen so far.
func (s *Server) ClearRanges() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ranges = nil
}

var rangeHeader = regexp.MustCompile(`^bytes=(\d+)-(\d*)$`)

// ServeHTTP serves the added files.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	f := s.files[r.URL.Path]
	s.ranges = append(s.ranges, r.Header.Get("Range"))
	delay := s.delay
	limit := int64(-1)
	if f != nil && f.dropAfter > 0 {
		limit, f.dropAfter = f.dropAfter, 0
	}
	s.mu.Unlock()

	if f == nil {
		http.NotFound(w, r)
		return
	}
	size := int64(len(f.body))
	start, end := int64(0), size-1
	status := http.StatusOK
	if !f.noRanges {
		w.Header().Set("Accept-Ranges", "bytes")
		if m := rangeHeader.FindStringSubmatch(r.Header.Get("Range")); m != nil {
			start, _ = strconv.ParseInt(m[1], 10, 64)
			if m[2] != "" {
				end, _ = strconv.ParseInt(m[2], 10, 64)
			}
			end = min(end, size-1)
			status = http.StatusPartialContent
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
		}
	}
	w.Header().Set("Content-Type", f.contentType)
	for name, values := range f.headers {
		w.Header()[name] = values
	}
	if f.filename != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", f.filename))
	}
	if !f.noLength {
		w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	}
	w.WriteHeader(status)

	flusher, _ := w.(http.Flusher)
	for at := start; at <= end; {
		next := min(at+chunkSize, end+1)
		if limit >= 0 && next-start > limit {
			return // a short body: the client sees the connection close early
		}
		if _, err := w.Write(f.body[at:next]); err != nil {
			return
		}
		at = next
		if delay > 0 {
			if flusher != nil {
				flusher.Flush()
			}
			select {
			case <-time.After(delay):
			case <-r.Context().Done():
				return
			}
		}
	}
}
