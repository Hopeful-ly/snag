// Package streamdl downloads videos from sites and HLS/DASH playlists by running yt-dlp.
package streamdl

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Roham1299/snag/internal/chapters"
	"github.com/Roham1299/snag/internal/cookies"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/platform"
	"github.com/Roham1299/snag/internal/speed"
	"github.com/Roham1299/snag/internal/tools"
	"github.com/Roham1299/snag/internal/watchlink"
	"github.com/Roham1299/snag/internal/ytdlp"
)

// stopGrace is how long yt-dlp gets to save its state and exit before it is killed.
const stopGrace = 5 * time.Second

// ErrLinkExpired means a stream's link stopped working part way.
var ErrLinkExpired = errors.New("the stream link expired: open the video's page and download the same " +
	"quality again, and Snag continues where it stopped")

// ErrNoYtDLP means yt-dlp isn't installed where Snag looks for it.
var ErrNoYtDLP = errors.New("yt-dlp is missing")

var expiredStatus = regexp.MustCompile(`HTTP Error (401|403|404|410)`)

// Runner downloads streams with yt-dlp. It implements engine.Runner.
type Runner struct {
	tools tools.Paths
}

// New returns a Runner that uses the given programs.
func New(t tools.Paths) *Runner {
	return &Runner{tools: t}
}

// Run downloads job's stream.
func (r *Runner) Run(ctx context.Context, job engine.Job, report engine.Reporter) error {
	if r.tools.YtDLP == "" {
		return ErrNoYtDLP
	}
	if err := os.MkdirAll(job.WorkDir, 0o700); err != nil {
		return err
	}
	parallel := max(1, job.Connections)
	s := &stream{
		job:      job,
		report:   report,
		programs: r.tools,
		path:     job.Path,
		parallel: parallel,
		estimate: sizeEstimate{hint: job.SizeHint, parallel: parallel},
	}
	return s.run(ctx)
}

// stream is one run of a download, which may span several yt-dlp runs (see apart.go).
type stream struct {
	job      engine.Job
	report   engine.Reporter
	programs tools.Paths

	path       string // "" until claimed
	claimedFor string
	linked     bool
	unlinkable bool
	writing    string
	final      string // the finished file in the work folder
	tags       string // see ytdlp.TagsOf

	chapters []chapters.Chapter // the whole video's, for a clip

	parallel           int
	estimate           sizeEstimate
	startedWithContent bool
	progressThisRun    atomic.Bool
	staged             atomic.Bool // a stage was reported since bytes last came

	reporting sync.Mutex
	cutSpeed  speed.Meter

	mu       sync.Mutex // guards what follows: players read it from other goroutines
	picture  string
	parts    []*part // picture and sound, when fetched apart
	readable bool
}

func (s *stream) run(ctx context.Context) error {
	s.startedWithContent = hasPieces(s.job.WorkDir)
	o := ytdlp.DownloadOptions{
		URL:          s.job.URL,
		Dir:          s.job.WorkDir,
		Name:         s.job.Filename,
		Format:       s.job.Format,
		Sort:         s.job.Sort,
		ExtractAudio: s.job.Audio,
		Clip:         s.job.Clip,
		Meta:         s.job.Meta,
		Connections:  s.parallel,
		LimitRate:    s.job.SpeedLimit,
		Headers:      s.job.Headers,
		Programs:     ytdlp.Programs{FFmpeg: s.programs.FFmpeg, Node: s.programs.Node},
		// Fix HLS's container while writing tags: one rewrite, not two (see fixContainer).
		FixInTags: isHLS(s.job.URL) && s.job.Audio == "" && !s.job.Clip.IsSet(),
	}
	if len(s.job.Cookies) > 0 {
		o.CookieFile = filepath.Join(s.job.WorkDir, "cookies.txt")
		if err := cookies.WriteFile(o.CookieFile, s.job.Cookies); err != nil {
			return err
		}
		defer os.Remove(o.CookieFile)
	}

	handle := s.handle
	switch {
	case o.Clip.IsSet():
		handle = s.handleClip
	case o.ExtractAudio == "" && ytdlp.Joined(o.Format):
		if err := s.fetchApart(ctx, &o); err != nil {
			return err
		}
	}
	if err := s.ytdlp(ctx, ytdlp.DownloadArgs(o), handle); err != nil {
		return err
	}
	if err := s.fixContainer(ctx); err != nil {
		return err
	}
	if s.job.Subs != "" {
		s.report.Stage("Fetching subtitles")
	}
	if err := s.fetchCaptions(ctx, o); err != nil {
		return err
	}
	if s.job.Meta.Tags.Cover != "" {
		s.report.Stage("Adding its cover")
	}
	if err := s.putCover(ctx); err != nil {
		return err
	}
	s.report.Stage("Moving it into place")
	return s.finish()
}

func (s *stream) ytdlp(ctx context.Context, args []string, handle func(ytdlp.Line)) error {
	return s.exec(ctx, args, handle, s.failure)
}

// exec runs yt-dlp, passing each output line to handle (if any); explain builds the error on
// failure from the exit and the last lines.
func (s *stream) exec(ctx context.Context, args []string, handle func(ytdlp.Line), explain func(exit error, tail []string) error) error {
	cmd := exec.Command(s.programs.YtDLP, args...)
	platform.StartGroup(cmd)
	cmd.WaitDelay = stopGrace
	output, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting yt-dlp: %w", err)
	}

	exited := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			stop(cmd, exited)
		case <-exited:
		}
	}()
	var tail []string
	var watching chan struct{} // closed to stop watchFinishing
	defer func() {
		if watching != nil {
			close(watching)
		}
	}()
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	scanner.Split(scanLines) // ffmpeg ends its progress lines with \r alone
	for scanner.Scan() {
		if scanner.Text() == "" {
			continue
		}
		line := ytdlp.Parse(scanner.Text())
		progress := line.Kind == ytdlp.LineProgress || line.Kind == ytdlp.LineCut
		if !ytdlp.Chatter(line) {
			s.report.Said(scanner.Text())
		}
		if stage, ok := ytdlp.StageOf(scanner.Text()); ok {
			s.staged.Store(true)
			s.report.Stage(stage)
			if watching != nil {
				close(watching)
			}
			watching = make(chan struct{})
			go s.watchFinishing(watching)
		} else if line.Kind == ytdlp.LineProgress && line.Progress.Status == "downloading" && s.staged.Swap(false) {
			s.report.Stage("") // the next part, after the first was fixed up
			if watching != nil {
				close(watching)
				watching = nil
			}
		}
		switch {
		case progress:
			if line.Progress.Downloaded > 0 {
				s.progressThisRun.Store(true)
			}
		case line.Kind == ytdlp.LineWarning:
			slog.Info("yt-dlp warning", "download", s.job.URL, "warning", line.Text)
		case line.Kind == ytdlp.LineTags:
			s.tags = line.Text // only the run that makes the whole file prints these
		}
		if !progress {
			tail = append(tail, scanner.Text())
			if len(tail) > tailLines {
				tail = tail[1:]
			}
		}
		if handle != nil {
			handle(line)
		}
	}
	err = cmd.Wait()
	close(exited)

	switch {
	case ctx.Err() != nil:
		return ctx.Err()
	case err != nil:
		return explain(err, tail)
	}
	return nil
}

func scanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		if data[i] == '\r' && i+1 < len(data) && data[i+1] == '\n' {
			return i + 2, data[:i], nil
		}
		if data[i] == '\r' && i+1 == len(data) && !atEOF {
			return 0, nil, nil // a \n may follow
		}
		return i + 1, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func stop(cmd *exec.Cmd, exited <-chan struct{}) {
	_ = platform.StopGroup(cmd, false)
	select {
	case <-exited:
	case <-time.After(stopGrace):
		_ = platform.StopGroup(cmd, true)
	}
}

func (s *stream) failure(exit error, tail []string) error {
	message := ytdlp.LastError(tail)
	if message == "" {
		message = fmt.Sprintf("yt-dlp failed (%v)", exit)
	}
	if expiredStatus.MatchString(message) && (s.progressThisRun.Load() || s.startedWithContent) {
		return ErrLinkExpired
	}
	if len(message) > 400 {
		message = message[:400]
	}
	return errors.New(message)
}

func (s *stream) finish() error {
	if s.final == "" || !sameDir(filepath.Dir(s.final), s.job.WorkDir) {
		return errors.New("yt-dlp didn't say where it saved the video")
	}
	info, err := os.Stat(s.final)
	if err != nil {
		return err
	}
	if err := s.claim(s.finalName()); err != nil {
		return err
	}
	// The final path was claimed by this download, so whatever is there is its own link.
	if err := watchlink.Finish(s.path, s.final, true); err != nil {
		return err
	}
	s.keepChapters()
	if err := s.keepCaptions(); err != nil {
		s.withoutCaptions(err)
	}
	if err := os.RemoveAll(s.job.WorkDir); err != nil {
		slog.Warn("couldn't delete a finished download's work folder", "dir", s.job.WorkDir, "err", err)
	}
	s.report.Progress(engine.Progress{Size: info.Size(), Done: info.Size(), Resumable: true})
	return nil
}

// finalName is the name from the list's template and tags, if any, else yt-dlp's.
func (s *stream) finalName() string {
	saved := filepath.Base(s.final)
	if !s.job.Naming.IsSet() || s.tags == "" {
		return saved
	}
	tags, err := ytdlp.TagsOf(s.tags)
	if err != nil {
		slog.Warn("couldn't name a download from its tags", "download", s.job.URL, "err", err)
		return saved
	}
	if name := s.job.Naming.Name(tags); name != "" {
		return name + filepath.Ext(saved)
	}
	return saved
}

// sameDir compares folders by identity, since Windows has short and long names for one folder.
func sameDir(a, b string) bool {
	infoA, errA := os.Stat(a)
	infoB, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(infoA, infoB)
}

func hasPieces(dir string) bool {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "cookies.txt" && e.Name() != ytdlp.ChoiceFile {
			return true
		}
	}
	return false
}
