package streamdl

import (
	"context"
	"log/slog"
	"time"

	"github.com/Roham1299/snag/internal/chapters"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/ytdlp"
)

// handleClip handles yt-dlp output while ffmpeg cuts a clip. A clip's file can't be played or
// resumed before it is finished, so it isn't offered for watching and a stopped clip starts over.
func (s *stream) handleClip(line ytdlp.Line) {
	switch line.Kind {
	case ytdlp.LineCut:
		s.cutProgress(line.Progress)
	case ytdlp.LineTitle:
		s.report.Titled(line.Text)
	case ytdlp.LineFinal:
		s.final = line.Text
	case ytdlp.LineChapters:
		s.chapters = chapters.Parse(line.Text)
	}
}

// keepChapters puts the video's chapters, cut to the clip, on a finished clip. Failure is only logged.
func (s *stream) keepChapters() {
	chs := chapters.Cut(s.chapters, s.job.Clip)
	if !s.job.Clip.IsSet() || len(chs) == 0 || s.path == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := chapters.Embed(ctx, s.programs.FFmpeg, s.path, chs); err != nil {
		slog.Warn("couldn't add chapters to a clip", "download", s.job.URL, "err", err)
	}
}

// cutProgress extrapolates the clip's size from ffmpeg's output once 5% is cut, using
// SizeHint before that.
func (s *stream) cutProgress(p ytdlp.Progress) {
	size, done := s.job.SizeHint, p.Downloaded
	if part := p.Cut / s.job.Clip.Length(); part >= 0.05 && done > 0 {
		size = int64(float64(done) / min(part, 1))
	}
	s.report.Progress(engine.Progress{
		Size:      max(size, done),
		Done:      done,
		Speed:     s.cutSpeed.Sample(done, time.Now()),
		Estimated: true,
	})
}
