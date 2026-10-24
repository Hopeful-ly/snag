package streamdl

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Roham1299/snag/internal/ytdlp"
)

// captionWaits are the pauses before retrying captions after an HTTP 429.
var captionWaits = []time.Duration{5 * time.Second, 15 * time.Second}

var (
	tooManyRequests = regexp.MustCompile(`HTTP Error 429`)
	youTube         = regexp.MustCompile(`(?i)(^|\.)(youtube\.com|youtube-nocookie\.com|youtu\.be)$`)
)

// fetchCaptions fetches captions after the video, so a site refusing them never costs the video.
// It only fails when stopped; captions that can't be had are noted instead.
func (s *stream) fetchCaptions(ctx context.Context, o ytdlp.DownloadOptions) error {
	if s.job.Subs == "" || s.job.SubURL != "" || s.job.SubText != "" || s.job.Audio != "" {
		return nil
	}
	o.InfoFile = "" // the info JSON's links may have expired by now
	args := ytdlp.CaptionArgs(o, s.job.Subs, s.job.SubAuto)
	for attempt := 0; ; attempt++ {
		err := s.exec(ctx, args, nil, captionFailure)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil && s.fetchedCaptions() == "" {
			err = errors.New("the site has none in this language")
		}
		if err == nil {
			return nil
		}
		if attempt == len(captionWaits) || !askAgain(s.job.URL, err) {
			s.withoutCaptions(err)
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(captionWaits[attempt]):
		}
	}
}

// askAgain reports whether a retry may help. YouTube's 429 for captions isn't rate limiting: it
// refuses them to anything but its own player.
func askAgain(address string, err error) bool {
	if !tooManyRequests.MatchString(err.Error()) {
		return false
	}
	u, perr := url.Parse(address)
	return perr != nil || !youTube.MatchString(u.Hostname())
}

func captionFailure(exit error, tail []string) error {
	return errors.New(cmp.Or(ytdlp.LastError(tail), exit.Error()))
}

func (s *stream) fetchedCaptions() string {
	found, _ := filepath.Glob(filepath.Join(s.job.WorkDir, ytdlp.CaptionsFile+".*"))
	for _, path := range found {
		if !strings.HasSuffix(path, ".part") {
			return path
		}
	}
	return ""
}

func (s *stream) fetchedCaptionText() string {
	path := s.fetchedCaptions()
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func (s *stream) withoutCaptions(err error) {
	slog.Warn("saved without captions", "download", s.job.URL, "err", err)
	why := err.Error()
	if tooManyRequests.MatchString(why) {
		// YouTube's player loads captions when they're turned on, and the extension then hands
		// Snag the player's.
		why = "the site refused them (too many requests); turn captions on in the player first"
	} else if len(why) > 160 {
		why = why[:160] + "…"
	}
	s.report.Note("Saved without subtitles: " + why)
}
