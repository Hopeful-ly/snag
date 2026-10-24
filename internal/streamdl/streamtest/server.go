// Package streamtest serves HLS streams for tests: a master playlist with three qualities, and
// long media playlists of junk pieces sent slowly, whose links can be made to expire.
package streamtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

const master = "#EXTM3U\n" +
	"#EXT-X-STREAM-INF:BANDWIDTH=5000000,RESOLUTION=1920x1080\nhi.m3u8\n" +
	"#EXT-X-STREAM-INF:BANDWIDTH=2500000,RESOLUTION=1280x720\nmid.m3u8\n" +
	"#EXT-X-STREAM-INF:BANDWIDTH=2000000,RESOLUTION=1280x720\nmid2.m3u8\n"

// Server serves a stream at any path ending in .m3u8 ("master.m3u8" is the master playlist), so
// each test can use its own path prefix.
type Server struct {
	*httptest.Server

	Pieces    int
	PieceSize int
	// Delay is the pause between each piece's 16 KiB chunks.
	Delay time.Duration

	mu        sync.Mutex
	requested []string
	completed map[string]bool
	expired   []string
}

// NewServer starts a server that stops when the test ends.
func NewServer(t testing.TB) *Server {
	s := &Server{Pieces: 200, PieceSize: 64 << 10, Delay: 100 * time.Millisecond, completed: map[string]bool{}}
	s.Server = httptest.NewServer(s)
	t.Cleanup(s.Close)
	return s
}

// Expire makes every address containing "/token/" answer 403 Forbidden.
func (s *Server) Expire(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expired = append(s.expired, "/"+token+"/")
}

// Requested returns every piece asked for under prefix, in order.
func (s *Server) Requested(prefix string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, p := range s.requested {
		if strings.HasPrefix(p, prefix) {
			out = append(out, p)
		}
	}
	return out
}

// Completed returns the pieces under prefix that were sent in full.
func (s *Server) Completed(prefix string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for p := range s.completed {
		if strings.HasPrefix(p, prefix) {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
}

// ServeHTTP serves the playlists and pieces.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	s.mu.Lock()
	expired := slices.ContainsFunc(s.expired, func(token string) bool { return strings.Contains(path, token) })
	s.mu.Unlock()
	switch {
	case expired:
		http.Error(w, "expired", http.StatusForbidden)
	case strings.HasSuffix(path, "master.m3u8"):
		s.playlist(w, master)
	case strings.HasSuffix(path, ".m3u8"):
		var b strings.Builder
		b.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:4\n#EXT-X-PLAYLIST-TYPE:VOD\n")
		for i := range s.Pieces {
			fmt.Fprintf(&b, "#EXTINF:4.0,\nseg%d.ts\n", i)
		}
		b.WriteString("#EXT-X-ENDLIST\n")
		s.playlist(w, b.String())
	case strings.HasSuffix(path, ".ts"):
		s.piece(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) playlist(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	_, _ = w.Write([]byte(body))
}

func (s *Server) piece(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.requested = append(s.requested, r.URL.Path)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Content-Length", fmt.Sprint(s.PieceSize))
	bit := make([]byte, 16<<10)
	flusher, _ := w.(http.Flusher)
	for sent := 0; sent < s.PieceSize; {
		n := min(len(bit), s.PieceSize-sent)
		if _, err := w.Write(bit[:n]); err != nil {
			return
		}
		sent += n
		if flusher != nil {
			flusher.Flush()
		}
		select {
		case <-time.After(s.Delay):
		case <-r.Context().Done():
			return
		}
	}
	s.mu.Lock()
	s.completed[r.URL.Path] = true
	s.mu.Unlock()
}

// StateIntact reports whether every .ytdl resume file in dir is valid JSON. Stopping yt-dlp
// mid-rewrite leaves one empty, and yt-dlp then restarts that download by design.
func StateIntact(dir string) bool {
	paths, _ := filepath.Glob(filepath.Join(dir, "*.ytdl"))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil || !json.Valid(data) {
			return false
		}
	}
	return true
}
