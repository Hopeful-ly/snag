package engine

import (
	"context"

	"github.com/Roham1299/snag/internal/checksum"
	"github.com/Roham1299/snag/internal/clip"
	"github.com/Roham1299/snag/internal/cookies"
	"github.com/Roham1299/snag/internal/meta"
	"github.com/Roham1299/snag/internal/naming"
	"github.com/Roham1299/snag/internal/ratelimit"
)

// Runner downloads one kind of thing; httpdl and streamdl implement it. Run returns when the file
// is at its final path, ctx is canceled, or it fails. Unfinished work stays in job.WorkDir so a
// later Run continues it; a watchlink at the final path may point into it while downloading.
type Runner interface {
	Run(ctx context.Context, job Job, report Reporter) error
}

// Job is what a runner needs to know about a download.
type Job struct {
	URL string
	// Resolve says URL is a page whose stream a browser has to find first (see pagedl).
	Resolve bool
	// Fallback is where to look when YouTube Music has no match for a song ("youtube" or
	// "soundcloud"); "" asks the user instead of searching.
	Fallback string
	Meta     meta.Label
	Naming   naming.Naming
	Headers  map[string]string
	Cookies  []cookies.Cookie
	Title    string
	Filename string // a name the user chose, or ""
	// Path is the final path once one was claimed, as when resuming; "" on a first run.
	Path        string
	WorkDir     string
	Connections int

	Format   string
	Sort     string
	Audio    string
	Subs     string
	Sidecar  bool
	SubAuto  bool
	SubURL   string
	SubText  string
	Clip     clip.Range
	SizeHint int64 // a size estimated in advance, 0 if none

	// Limit is the speed limit shared with the rest of its queue, waited on for what is read; nil
	// for none.
	Limit *ratelimit.Limiter
	// SpeedLimit is this download's share of that limit, in bytes a second, for runners that can
	// only be given a fixed rate (yt-dlp); 0 for none.
	SpeedLimit int64
}

// Reporter is how a runner tells the engine what it learns.
type Reporter interface {
	// ClaimPath turns a file name into the download's final path, unique in its folder. Runners
	// call it when the job has no path yet, before putting anything there, and again if the
	// name turns out different (a stream's container), which gives up the earlier claim.
	ClaimPath(name string) (string, error)
	// Titled reports the title the source gives, for downloads added without one.
	Titled(title string)
	// Progress reports the download's state, a few times a second.
	Progress(p Progress)
	// Readable offers the file being written for watching it before it is finished, once it
	// is open. Runners whose file can't be read that way don't call it.
	Readable(r Readable)
	// Note reports something that isn't a failure, such as missing subtitles; it shows until the
	// download runs again.
	Note(text string)
	// Checksum reports a whole-file checksum the source sent, for checking the finished file.
	Checksum(sum checksum.Sum)
	// Stage says what the run is doing besides fetching bytes, such as muxing; "" once it fetches
	// again.
	Stage(text string)
	// StageProgress reports how far the current stage is, when known; a new stage starts with none.
	StageProgress(done, of int64)
	// Said passes on a line the run's program printed, or a note about the run, for its details.
	Said(text string)
}

// Readable is a running download's file as it fills. Its methods are called from other
// goroutines, never while the engine holds its lock.
type Readable interface {
	// Path is the file being written, with every byte at its final offset.
	Path() string
	// Have returns how many bytes from off are in the file already: 0 when off's byte isn't.
	Have(off int64) int64
	// Want fetches the bytes from off first: a player jumped there and waits for them.
	Want(off int64)
}

// Tracks is a Readable that may be a picture whose sound is a separate file until the download
// finishes and joins them (sites like YouTube send them apart); Path and Have are the
// picture's then, and Have is 0 until the sound has the start a player needs with it.
type Tracks interface {
	Readable
	// Sound is the sound's file as it fills ("" until it begins), and whether there is a
	// separate sound at all. Like Path, it changes name when it is complete.
	Sound() (path string, separate bool)
}

// Progress is a runner's report on a download.
type Progress struct {
	Size        int64 // 0 when unknown
	Done        int64
	Speed       int64 // bytes per second
	Connections int
	Resumable   bool
	Estimated   bool  // Size is a guess
	Playable    int64 // bytes from the start already in the file, for watching it
	// ToPlay is how many bytes must still arrive before it can be watched, when the runner
	// knows better than Playable (a separate sound needs its start too); 0 when it can be.
	ToPlay int64
	// Pieces and PiecesDone count a stream's pieces; 0 when it isn't in pieces.
	Pieces, PiecesDone int
}
