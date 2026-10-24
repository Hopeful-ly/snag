package streamdl

import (
	"context"
	"log/slog"
	"os"

	"github.com/Roham1299/snag/internal/cover"
)

// putCover embeds meta.Tags.Cover in the song before it is moved into place. Failing to get the
// cover only adds a note; only cancellation is an error.
func (s *stream) putCover(ctx context.Context) error {
	address := s.job.Meta.Tags.Cover
	if address == "" || s.final == "" {
		return nil
	}
	err := func() error {
		picture, err := cover.Fetch(ctx, nil, address, s.job.WorkDir)
		if err != nil {
			return err
		}
		defer os.Remove(picture)
		return cover.Embed(ctx, s.programs.FFmpeg, s.final, picture)
	}()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		slog.Info("couldn't put the cover in", "download", s.job.URL, "cover", address, "err", err)
		s.report.Note("Saved without its cover: " + err.Error())
	}
	return nil
}
