package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/Roham1299/snag/internal/checksum"
)

// CheckResult says where checking a finished file against its checksum stands.
type CheckResult string

// The states of a check.
const (
	CheckNone     CheckResult = ""        // a checksum is known or not, but nothing was checked
	CheckLooking  CheckResult = "looking" // looking online for its checksum
	CheckChecking CheckResult = "checking"
	CheckMatch    CheckResult = "match"
	CheckMismatch CheckResult = "mismatch"
	CheckFailed   CheckResult = "failed" // the file couldn't be read
)

// Where a checksum came from.
const (
	FromYou    = "you" // the user pasted it
	FromPage   = "page"
	FromServer = "server" // the file's server sent it with the file
	FromGitHub = "github" // GitHub's record of a release's files
	FromFile   = "file"   // a checksum file next to it, or one the page links to
)

// Check is a finished file's checksum, where it came from, and checking the file against it.
type Check struct {
	Algo   checksum.Algo `json:"algo,omitempty"`
	Want   string        `json:"want,omitempty"` // the checksum, lower-case hex; "" while none is known
	From   string        `json:"from,omitempty"`
	Result CheckResult   `json:"result,omitempty"`
	Got    string        `json:"got,omitempty"`
	Error  string        `json:"error,omitempty"`
	Read   int64         `json:"read,omitempty"` // bytes read so far, while checking
}

// Found reports whether a checksum is known to check against.
func (c Check) Found() bool { return c.Want != "" }

func (c *Check) use(s checksum.Sum, from string) {
	*c = Check{Algo: s.Algo, Want: s.Hex, From: from}
}

// ChecksumFinder looks online for a finished download's checksum; a zero Sum means none was found.
type ChecksumFinder func(ctx context.Context, d Download) (sum checksum.Sum, from string, err error)

const findTimeout = 45 * time.Second

// Errors from checking.
var (
	ErrNotFinished = errors.New("it isn't finished yet")
	ErrNoChecksum  = errors.New("there's no checksum to check it against: paste one")
	ErrBadChecksum = errors.New("that isn't a checksum Snag knows (MD5, SHA-1, SHA-256, SHA-384 or SHA-512)")
)

// lookForChecksum looks online for d's checksum in the background when none is known. d keeps
// the cookies the site wants. The lock must be held.
func (e *Engine) lookForChecksum(en *entry, d Download) {
	if e.findChecksum == nil || d.Check.Found() || d.Kind != KindHTTP || d.Path == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), findTimeout)
	en.checkCancel = cancel
	en.d.Check.Result = CheckLooking
	go func() {
		defer cancel()
		sum, from, err := e.findChecksum(ctx, d)
		e.mu.Lock()
		defer e.mu.Unlock()
		en.checkCancel = nil
		if en.d.Check.Result != CheckLooking {
			return // the user pasted one meanwhile, or it is being removed
		}
		en.d.Check.Result = CheckNone
		if err != nil {
			e.log.Info("couldn't look for a checksum", "id", d.ID, "err", err)
		} else if !sum.IsZero() {
			en.d.Check.use(sum, from)
			e.log.Info("found a checksum", "id", d.ID, "from", from, "algo", sum.Algo)
		}
		e.markDirty()
		e.changed(en, true)
	}()
}

// Check checks a finished download's file against its checksum in the background; it reads the
// whole file, so only on request. text is a checksum in any common notation; "" uses the known one.
func (e *Engine) Check(id, text string) error {
	var given checksum.Sum
	if text != "" {
		sum, ok := checksum.Parse(text)
		if !ok {
			return ErrBadChecksum
		}
		given = sum
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	en := e.find(id)
	if en == nil {
		return ErrNotFound
	}
	if en.d.Status != StatusDone {
		return ErrNotFinished
	}
	if !given.IsZero() {
		if en.checkCancel != nil {
			en.checkCancel() // a look or a check for another checksum
			en.checkCancel = nil
		}
		en.d.Check.use(given, FromYou)
	}
	c := &en.d.Check
	if !c.Found() {
		return ErrNoChecksum
	}
	if c.Result == CheckChecking {
		return nil
	}
	if _, err := os.Stat(en.d.Path); err != nil {
		return errors.New("the file isn't there any more")
	}
	ctx, cancel := context.WithCancel(context.Background())
	en.checkCancel = cancel
	c.Result, c.Got, c.Error, c.Read = CheckChecking, "", "", 0
	e.markDirty()
	e.changed(en, true)
	path, algo, want := en.d.Path, c.Algo, c.Want
	go func() {
		defer cancel()
		got, err := checksum.File(ctx, path, algo, func(read int64) {
			e.mu.Lock()
			defer e.mu.Unlock()
			if en.d.Check.Result == CheckChecking {
				en.d.Check.Read = read
				e.changed(en, false)
			}
		})
		e.mu.Lock()
		defer e.mu.Unlock()
		c := &en.d.Check
		if ctx.Err() != nil || c.Result != CheckChecking || c.Want != want {
			return // removed, shut down, or another checksum given meanwhile
		}
		en.checkCancel = nil
		c.Read = 0
		switch {
		case err != nil:
			c.Result, c.Error = CheckFailed, err.Error()
		case got == want:
			c.Result, c.Got = CheckMatch, got
		default:
			c.Result, c.Got = CheckMismatch, got
		}
		e.log.Info("checked a file", "id", en.d.ID, "result", c.Result, "algo", algo)
		e.markDirty()
		e.changed(en, true)
	}()
	return nil
}

// stopChecking cancels any checksum lookup or check. The lock must be held.
func (e *Engine) stopChecking(en *entry) {
	if en.checkCancel != nil {
		en.checkCancel()
		en.checkCancel = nil
	}
	if r := en.d.Check.Result; r == CheckLooking || r == CheckChecking {
		en.d.Check.Result, en.d.Check.Read = CheckNone, 0
	}
}

// Redownload downloads a finished file again to the same path. The old file is replaced only once
// the new one is complete, and stays if the new download is removed; the checksum carries over.
func (e *Engine) Redownload(id string) (Download, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	en := e.find(id)
	if en == nil {
		return Download{}, ErrNotFound
	}
	old := en.d
	if old.Status != StatusDone {
		return Download{}, ErrNotFinished
	}
	req := AddRequest{
		URL: old.URL, Page: old.Page, Title: old.Title, Label: old.Label, Kind: old.Kind,
		Format: old.Format, Sort: old.Sort, Audio: old.Audio, Subs: old.Subs, Sidecar: old.Sidecar,
		SubAuto: old.SubAuto, SubURL: old.SubURL, SubText: old.SubText, Clip: old.Clip,
		Resolve: old.Resolve, Meta: old.Meta, Naming: old.Naming, Headers: old.Headers,
		Directory: old.Directory, Queue: old.Queue, Confirmed: true,
		ChecksumFiles: old.ChecksumFiles,
	}
	if old.Path != "" {
		req.Filename = filepath.Base(old.Path)
	}
	d, _, err := e.add(req, false)
	if err != nil {
		return Download{}, err
	}
	fresh := e.find(d.ID)
	if old.Path != "" {
		fresh.d.Replace = []string{old.Path}
	}
	if old.Check.Found() {
		fresh.d.Check = Check{Algo: old.Check.Algo, Want: old.Check.Want, From: old.Check.From}
	}
	e.stopChecking(en)
	e.entries = slices.DeleteFunc(e.entries, func(other *entry) bool { return other == en })
	e.markDirty()
	e.emit(EventRemoved, Download{ID: old.ID})
	e.emit(EventAdded, fresh.d)
	e.schedule()
	return fresh.d, nil
}
