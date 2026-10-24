package streamdl

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/subs"
	"github.com/Roham1299/snag/internal/watchlink"
	"github.com/Roham1299/snag/internal/ytdlp"
)

const tailLines = 12

func (s *stream) handle(line ytdlp.Line) {
	if s.apart() {
		if line.Kind == ytdlp.LineFinal {
			s.final = line.Text
		}
		return
	}
	switch line.Kind {
	case ytdlp.LineProgress:
		s.progress(line.Progress)
	case ytdlp.LineTitle:
		s.report.Titled(line.Text)
	case ytdlp.LineDestination:
		s.setPicture(line.Text)
		s.writing = line.Text
		s.linkUp()
	case ytdlp.LineAlready:
		s.setPicture(line.Text)
	case ytdlp.LineFinal:
		s.final = line.Text
	}
}

func (s *stream) progress(p ytdlp.Progress) {
	// When resuming, yt-dlp may count only this run's bytes.
	downloaded := max(p.Downloaded, fileSize(s.writing+".part"))
	size, guess := s.estimate.update(downloaded, p.Total, p.Fragment, p.Fragments)
	s.report.Progress(engine.Progress{
		Size:      size,
		Done:      downloaded,
		Speed:     p.Speed,
		Resumable: true,
		Estimated: guess,
		Playable:  s.Have(0),
		ToPlay:    s.toPlay(),

		Pieces:     p.Fragments,
		PiecesDone: p.Fragment,
	})
	s.linkUp()
}

// linkUp links the file yt-dlp is writing at the final name so it can be watched mid-download.
// A hard link needs the file to exist, so this is retried on each progress update.
func (s *stream) linkUp() {
	if s.writing == "" || s.linked || s.unlinkable {
		return
	}
	part := s.writing + ".part"
	if _, err := os.Stat(part); err != nil {
		return
	}
	if err := s.claim(filepath.Base(s.writing)); err != nil {
		slog.Warn("couldn't claim a name to watch a download by", "download", s.job.URL, "err", err)
		s.unlinkable = true
		return
	}
	s.linked = watchlink.Make(s.path, part)
}

// claim gets the final path for name, keeping an earlier claim made for the same name (as on
// resume) and giving it up otherwise.
func (s *stream) claim(name string) error {
	if s.path != "" {
		sameFile := s.claimedFor == name || (s.claimedFor == "" && filepath.Ext(s.path) == filepath.Ext(name))
		if sameFile {
			s.claimedFor = name
			return nil
		}
		// The earlier path was claimed by this download, so what is there is its own link.
		if info, err := os.Lstat(s.path); err == nil && (info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
			_ = os.Remove(s.path)
		}
		s.linked = false
	}
	path, err := s.report.ClaimPath(name)
	if err != nil {
		return err
	}
	s.path, s.claimedFor = path, name
	return nil
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// keepCaptions attaches the requested captions to the finished video, cut to the clip if any.
func (s *stream) keepCaptions() error {
	if s.path == "" || s.job.Subs == "" {
		return nil
	}
	text := s.job.SubText
	// A whole-video sidecar is copied as yt-dlp named it; anything else goes through Attach.
	if text == "" && s.job.SubURL == "" && (s.job.Clip.IsSet() || !s.job.Sidecar) {
		text = s.fetchedCaptionText()
	}
	if s.job.SubURL == "" && text == "" {
		return s.keepSidecar()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return subs.Attach(ctx, subs.Request{
		URL:     s.job.SubURL,
		Text:    text,
		Video:   s.path,
		Lang:    s.job.Subs,
		Sidecar: s.job.Sidecar,
		Clip:    s.job.Clip,
		Headers: s.job.Headers,
		Cookies: s.job.Cookies,
		FFmpeg:  s.programs.FFmpeg,
		WorkDir: s.job.WorkDir,
	})
}

func (s *stream) keepSidecar() error {
	if s.job.Subs == "" || !s.job.Sidecar || s.path == "" {
		return nil
	}
	entries, err := os.ReadDir(s.job.WorkDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !isSubtitleFile(e.Name()) {
			continue
		}
		src := filepath.Join(s.job.WorkDir, e.Name())
		dest := sidecarDest(s.path, e.Name())
		if err := copyFile(src, dest); err != nil {
			return err
		}
	}
	return nil
}

func isSubtitleFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".srt", ".vtt", ".ass", ".ssa":
		return true
	}
	return false
}

func sidecarDest(video, sub string) string {
	stem := strings.TrimSuffix(video, filepath.Ext(video))
	base := filepath.Base(sub)
	i := strings.Index(base, ".")
	if i < 0 {
		return stem + filepath.Ext(base)
	}
	return stem + base[i:]
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
