package watch

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/platform"
)

// Growing downloads have no known size, so they are served like a live broadcast, without seeking.
// Separate video and audio are joined by ffmpeg as they fill: the audio reaches it through a
// one-time private feed URL on this server.

func (s *Server) serveLive(w http.ResponseWriter, r *http.Request, d engine.Download) {
	a, ok := s.waitForStart(r.Context(), d.ID)
	if !ok {
		http.Error(w, "this download can't be watched", http.StatusGone)
		return
	}
	if a.Done {
		s.serveFile(w, r, d) // it finished while we waited
		return
	}
	picture := &pinned{ctx: r.Context(), src: s.src, id: d.ID, path: a.Path, track: pictureTrack, wait: s.wait}
	defer picture.close()
	ts := isTransportStream(a.Path) || (a.Separate && isTransportStream(a.Sound))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Accept-Ranges", "none")
	if !a.Separate {
		w.Header().Set("Content-Type", liveType(ts, contentType(d.Name())))
		if r.Method == http.MethodHead {
			return
		}
		_, _ = io.Copy(flusher{w}, picture)
		return
	}

	if s.ffmpeg == "" {
		http.Error(w, "joining the picture and sound needs ffmpeg", http.StatusServiceUnavailable)
		return
	}
	format, kind := "matroska", "video/x-matroska"
	if ts {
		format, kind = "mpegts", "video/mp2t" // TS sound carries its settings only in its packets
	}
	w.Header().Set("Content-Type", kind)
	if r.Method == http.MethodHead {
		return
	}
	sound := &pinned{ctx: r.Context(), src: s.src, id: d.ID, path: a.Sound, track: soundTrack, wait: s.wait}
	defer sound.close()
	feed := s.addFeed(sound)
	defer s.feeds.Delete(feed)
	join := exec.CommandContext(r.Context(), s.ffmpeg, "-hide_banner", "-loglevel", "error",
		"-i", "pipe:0", "-seekable", "0", "-i", s.feedURL(feed),
		"-map", "0", "-map", "1", "-c", "copy", "-f", format, "pipe:1")
	platform.StartGroup(join) // no console window on Windows
	join.Stdin, join.Stdout = picture, flusher{w}
	if err := join.Run(); err != nil && r.Context().Err() == nil {
		s.log.Info("joining a video's picture and sound stopped", "download", d.ID, "err", err)
	}
}

// waitForStart waits for the video's (and separate audio's) start. ok is false if the download failed or the player left.
func (s *Server) waitForStart(ctx context.Context, id string) (engine.Available, bool) {
	for {
		a, err := s.src.Available(id, 0)
		if err != nil {
			return a, false
		}
		if a.Done || (a.Path != "" && a.N > 0 && (!a.Separate || a.Sound != "")) {
			return a, true
		}
		select {
		case <-ctx.Done():
			return a, false
		case <-time.After(s.wait):
		}
	}
}

// pinned reads a growing file through one handle until it is complete, so reading survives
// yt-dlp renaming or deleting the file as it finishes.
type pinned struct {
	ctx   context.Context
	src   Source
	id    string
	path  string
	track func(engine.Available) string // the file's current path
	wait  time.Duration
	file  *os.File
	off   int64
}

func pictureTrack(a engine.Available) string { return a.Path }
func soundTrack(a engine.Available) string   { return a.Sound }

func (p *pinned) Read(b []byte) (int, error) {
	if p.file == nil {
		f, err := platform.OpenShared(p.path)
		if err != nil {
			return 0, err
		}
		p.file = f
	}
	complete := false
	for {
		info, err := p.file.Stat()
		if err != nil {
			return 0, err
		}
		if p.off < info.Size() {
			n, err := p.file.ReadAt(b[:min(int64(len(b)), info.Size()-p.off)], p.off)
			p.off += int64(n)
			if errors.Is(err, io.EOF) {
				err = nil
			}
			return n, err
		}
		if complete {
			return 0, io.EOF
		}
		a, err := p.src.Available(p.id, 0)
		if err != nil {
			return 0, err
		}
		if a.Done || p.track(a) != p.path {
			complete = true // look at the size once more, then end
			continue
		}
		select {
		case <-p.ctx.Done():
			return 0, p.ctx.Err()
		case <-time.After(p.wait):
		}
	}
}

func (p *pinned) close() {
	if p.file != nil {
		_ = p.file.Close()
	}
}

// addFeed registers r for ffmpeg to read at feedURL(key).
func (s *Server) addFeed(r io.Reader) string {
	key := make([]byte, 16)
	_, _ = rand.Read(key)
	feed := hex.EncodeToString(key)
	s.feeds.Store(feed, r)
	return feed
}

func (s *Server) feedURL(feed string) string {
	return "http://" + s.ln.Addr().String() + "/" + s.secret + "/feed/" + feed
}

// serveFeed serves a feed once, to the ffmpeg it was made for.
func (s *Server) serveFeed(w http.ResponseWriter, r *http.Request, feed string) {
	v, ok := s.feeds.LoadAndDelete(feed)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(flusher{w}, v.(io.Reader))
}

type flusher struct{ w http.ResponseWriter }

func (f flusher) Write(b []byte) (int, error) {
	n, err := f.w.Write(b)
	_ = http.NewResponseController(f.w).Flush()
	return n, err
}

// isTransportStream reports whether a file starts with the MPEG-TS sync byte, as HLS streams do.
func isTransportStream(path string) bool {
	f, err := platform.OpenShared(path)
	if err != nil {
		return false
	}
	defer f.Close()
	first := make([]byte, 1)
	_, err = f.ReadAt(first, 0)
	return err == nil && first[0] == 0x47
}

func liveType(ts bool, byName string) string {
	if ts {
		return "video/mp2t"
	}
	return byName
}
