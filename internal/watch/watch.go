// Package watch serves downloads to video players over loopback while they download; reads of
// bytes not yet arrived wait for them.
//
// URLs carry a secret a web page can't learn, and the Host header is checked against DNS rebinding.
package watch

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/platform"
)

// Source provides downloads and their bytes; the engine implements it.
type Source interface {
	Get(id string) (engine.Download, error)
	Available(id string, off int64) (engine.Available, error)
	// Want prioritizes the bytes from off, which a player is waiting for.
	Want(id string, off int64)
}

const waitInterval = 200 * time.Millisecond

// Server serves downloads to players. It is safe for concurrent use.
type Server struct {
	src    Source
	ffmpeg string // "" for none
	secret string
	ln     net.Listener
	http   *http.Server
	log    *slog.Logger
	wait   time.Duration
	feeds  sync.Map // see addFeed
}

// Start listens on a free loopback port. Without ffmpeg, downloads with separate video and audio
// can only be watched once finished.
func Start(src Source, ffmpeg string, log *slog.Logger) (*Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	secret := make([]byte, 16)
	_, _ = rand.Read(secret)
	s := &Server{src: src, ffmpeg: ffmpeg, secret: hex.EncodeToString(secret), ln: ln, log: log, wait: waitInterval}
	s.http = &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := s.http.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Warn("the watch server stopped", "err", err)
		}
	}()
	return s, nil
}

// Close stops the server and ends every stream.
func (s *Server) Close() error {
	return s.http.Close()
}

// URL is where a player can watch d. It ends in the file name so players show the title and detect the format.
func (s *Server) URL(d engine.Download) string {
	return "http://" + s.ln.Addr().String() + "/" + s.secret + "/" + d.ID + "/" + url.PathEscape(d.Name())
}

// ServeHTTP checks the Host header (against DNS rebinding) and the secret, in constant time.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Host != s.ln.Addr().String() {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 3)
	if len(parts) < 2 || subtle.ConstantTimeCompare([]byte(parts[0]), []byte(s.secret)) != 1 {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if parts[1] == "feed" && len(parts) == 3 {
		s.serveFeed(w, r, parts[2])
		return
	}
	d, err := s.src.Get(parts[1])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a, err := s.src.Available(d.ID, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusGone)
		return
	}
	if a.Live && !a.Done {
		s.serveLive(w, r, d)
		return
	}
	s.serveFile(w, r, d)
}

// serveFile serves a file whose size is known, so players can seek.
func (s *Server) serveFile(w http.ResponseWriter, r *http.Request, d engine.Download) {
	if d.Size <= 0 || d.Estimated {
		// A player needs the length to seek; a download that doesn't know it yet can't be served.
		http.Error(w, "this download's size isn't known yet", http.StatusServiceUnavailable)
		return
	}

	file := &growing{ctx: r.Context(), src: s.src, id: d.ID, size: d.Size, wait: s.wait, wanted: -1}
	defer file.close()
	w.Header().Set("Content-Type", contentType(d.Name()))
	w.Header().Set("Cache-Control", "no-store")
	// ServeContent answers ranges, HEAD and conditional requests; reads wait for missing bytes.
	http.ServeContent(w, r, d.Name(), time.Time{}, file)
}

var videoTypes = map[string]string{
	".mp4": "video/mp4", ".m4v": "video/mp4", ".mov": "video/quicktime", ".webm": "video/webm",
	".mkv": "video/x-matroska", ".avi": "video/x-msvideo", ".ts": "video/mp2t", ".flv": "video/x-flv",
	".wmv": "video/x-ms-wmv", ".mpg": "video/mpeg", ".mpeg": "video/mpeg", ".ogv": "video/ogg",
	".3gp": "video/3gpp", ".mp3": "audio/mpeg", ".m4a": "audio/mp4", ".ogg": "audio/ogg",
	".opus": "audio/ogg", ".flac": "audio/flac", ".wav": "audio/wav", ".aac": "audio/aac",
}

func contentType(name string) string {
	if t, ok := videoTypes[strings.ToLower(path.Ext(name))]; ok {
		return t
	}
	return "application/octet-stream"
}

// growing reads a filling file; reads past what has arrived wait until it does, the player leaves, or the download fails.
type growing struct {
	ctx  context.Context
	src  Source
	id   string
	size int64
	off  int64
	wait time.Duration

	file *os.File
	path string

	wanted int64 // the offset last asked for first, so each wait asks once; -1 for none
}

func (g *growing) Read(p []byte) (int, error) {
	if g.off >= g.size {
		return 0, io.EOF
	}
	for {
		a, err := g.src.Available(g.id, g.off)
		if err != nil {
			return 0, err
		}
		if a.N > 0 && a.Path != "" {
			n, err := g.readAt(p[:min(int64(len(p)), a.N)], a.Path)
			if n > 0 {
				return n, nil
			}
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return 0, err
			}
			// The file was renamed under us as the download finished: look again.
		}
		if a.N == 0 && g.wanted != g.off {
			g.wanted = g.off
			g.src.Want(g.id, g.off)
		}
		select {
		case <-g.ctx.Done():
			return 0, g.ctx.Err()
		case <-time.After(g.wait):
		}
	}
}

// readAt reopens the file when path has changed.
func (g *growing) readAt(p []byte, path string) (int, error) {
	if g.path != path {
		g.close()
		f, err := platform.OpenShared(filepath.Clean(path))
		if err != nil {
			return 0, err
		}
		g.file, g.path = f, path
	}
	n, err := g.file.ReadAt(p, g.off)
	g.off += int64(n)
	if errors.Is(err, io.EOF) {
		err = nil // some systems don't preallocate the final size
	}
	return n, err
}

func (g *growing) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += g.off
	case io.SeekEnd:
		offset += g.size
	default:
		return 0, errors.New("bad whence")
	}
	if offset < 0 {
		return 0, errors.New("negative position")
	}
	g.off = offset
	return offset, nil
}

func (g *growing) close() {
	if g.file != nil {
		_ = g.file.Close()
		g.file, g.path = nil, ""
	}
}
