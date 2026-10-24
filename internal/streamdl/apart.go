package streamdl

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/ytdlp"
)

// yt-dlp fetches a video's parts one after the other, so on a slow line watching waited minutes
// for the sound. Instead yt-dlp runs once to choose formats and write the info JSON, once per part
// concurrently from that JSON, and once more to join the parts it then finds in place.

type part struct {
	s        *stream
	format   ytdlp.Format
	estimate sizeEstimate

	// Guarded by s.mu.
	file      string // "" until yt-dlp names it
	done      int64
	size      int64 // 0 while unknown
	estimated bool
	speed     int64
	finished  bool
}

// fetchApart chooses formats and, for a picture and a sound, fetches both at once. o is left set
// for the run that downloads the choice or joins its parts.
func (s *stream) fetchApart(ctx context.Context, o *ytdlp.DownloadOptions) error {
	choiceFile := filepath.Join(s.job.WorkDir, ytdlp.ChoiceFile)
	// yt-dlp would reuse an earlier run's info JSON, whose links may have expired.
	if err := os.Remove(choiceFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	err := s.ytdlp(ctx, ytdlp.ChooseArgs(*o), func(line ytdlp.Line) {
		if line.Kind == ytdlp.LineTitle {
			s.report.Titled(line.Text)
		}
	})
	if err != nil {
		return err
	}
	choice, err := ytdlp.ReadChoice(choiceFile)
	if err != nil {
		return err
	}
	ids := make([]string, len(choice.Formats))
	for i, f := range choice.Formats {
		ids[i] = f.FormatID
	}
	o.InfoFile, o.Format = choiceFile, strings.Join(ids, "+")
	if len(choice.Formats) != 2 {
		return nil
	}

	picture, sound := s.newPart(choice.Formats[0]), s.newPart(choice.Formats[1])
	if picture.format.Sound() {
		picture, sound = sound, picture
	}
	s.mu.Lock()
	s.parts = []*part{picture, sound}
	s.mu.Unlock()

	pictureConnections := max(1, s.parallel*2/3)
	connections := map[*part]int{picture: pictureConnections, sound: max(1, s.parallel-pictureConnections)}
	partCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errs := make(chan error, 2)
	for p, n := range connections {
		po := *o
		po.Part, po.Format, po.Connections = true, p.format.FormatID, n
		if o.LimitRate > 0 {
			// The speed limit is split in proportion to connections.
			total := connections[picture] + connections[sound]
			po.LimitRate = max(o.LimitRate*int64(n)/int64(total), 1)
		}
		go func() {
			err := s.ytdlp(partCtx, ytdlp.DownloadArgs(po), p.handle)
			if err != nil {
				cancel()
			}
			errs <- err
		}()
	}
	var failed error
	for range connections {
		// Prefer the failed part's error over the other's context.Canceled.
		if err := <-errs; err != nil && (failed == nil || errors.Is(failed, context.Canceled)) {
			failed = err
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return failed
}

func (s *stream) newPart(f ytdlp.Format) *part {
	return &part{s: s, format: f, size: f.Size(), estimated: true, estimate: sizeEstimate{parallel: s.parallel}}
}

func (s *stream) apart() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.parts) > 0
}

func (p *part) handle(line ytdlp.Line) {
	s := p.s
	switch line.Kind {
	case ytdlp.LineDestination:
		s.mu.Lock()
		p.file = line.Text
		s.mu.Unlock()
		s.offer()
	case ytdlp.LineAlready:
		n := fileSize(line.Text)
		s.mu.Lock()
		p.file, p.done, p.size, p.estimated, p.speed, p.finished = line.Text, n, n, false, 0, true
		s.mu.Unlock()
		s.offer()
		s.reportParts()
	case ytdlp.LineProgress:
		pr := line.Progress
		s.mu.Lock()
		// When resuming, yt-dlp may count only this run's bytes.
		downloaded := max(pr.Downloaded, fileSize(p.file+".part"))
		size, guess := p.estimate.update(downloaded, pr.Total, pr.Fragment, pr.Fragments)
		if size == 0 && p.format.Size() > 0 {
			size, guess = max(p.format.Size(), downloaded), true
		}
		p.done, p.size, p.estimated, p.speed = downloaded, size, guess, pr.Speed
		if pr.Status == "finished" {
			p.size, p.estimated, p.speed, p.finished = downloaded, false, 0, true
		}
		s.mu.Unlock()
		s.reportParts()
	}
}

func (s *stream) reportParts() {
	s.reporting.Lock() // so an older report never lands last
	defer s.reporting.Unlock()
	p := engine.Progress{Resumable: true}
	unknown := false
	s.mu.Lock()
	for _, part := range s.parts {
		p.Size += part.size
		p.Done += part.done
		p.Speed += part.speed
		p.Estimated = p.Estimated || part.estimated
		unknown = unknown || part.size == 0
	}
	s.mu.Unlock()
	if unknown {
		p.Size = 0
	}
	p.Playable, p.ToPlay = s.Have(0), s.toPlay()
	s.report.Progress(p)
}
