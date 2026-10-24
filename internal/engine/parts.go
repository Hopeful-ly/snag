package engine

import (
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// partsFolder is the hidden folder in a download folder on another drive.
const partsFolder = ".snag"

// placeParts picks where a download keeps its parts on its first run: Snag's work folder if on
// the same drive, else a hidden folder beside the download, so the file can be hard-linked at its
// final name and finishing is a rename. The choice is kept for life.
func (e *Engine) placeParts(d *Download) {
	if d.WorkIn != "" || e.sameDrive == nil || d.Directory == "" {
		return
	}
	if _, err := os.Stat(filepath.Join(e.workRoot, d.Work)); err == nil {
		return // started before: its parts are in the work root
	}
	if e.sameDrive(e.workRoot, d.Directory) {
		return
	}
	in := filepath.Join(d.Directory, partsFolder)
	if err := os.MkdirAll(in, 0o700); err != nil {
		e.log.Warn("can't keep parts on the download's drive; finishing will copy it", "dir", in, "err", err)
		return
	}
	if e.hide != nil {
		if err := e.hide(in); err != nil {
			e.log.Debug("couldn't hide the parts folder", "dir", in, "err", err)
		}
	}
	d.WorkIn = in
}

// tidyParts removes a download's parts folder on another drive once nothing is left in it.
func tidyParts(d Download) {
	if d.WorkIn != "" {
		_ = os.Remove(d.WorkIn) // only if empty: other downloads may still be using it
	}
}

// How long deleting a work folder is tried for.
var (
	removeTries = 10
	removeRetry = 500 * time.Millisecond
)

// removeWork deletes a work folder, trying again for a while: on Windows a folder can't go while
// a program that wrote in it (yt-dlp, ffmpeg, a player) still has a file open.
func removeWork(dir string) error {
	var err error
	for range removeTries {
		if err = os.RemoveAll(dir); err == nil {
			return nil
		}
		time.Sleep(removeRetry)
	}
	return err
}

// tidyFinished deletes what a finished download left in a parts folder on another drive, which
// its runner couldn't, and the folder once it is empty.
func (e *Engine) tidyFinished(d Download) {
	if d.WorkIn == "" || filepath.Base(d.WorkIn) != partsFolder || !workName.MatchString(d.Work) {
		return
	}
	if err := removeWork(e.workDir(d)); err != nil {
		e.log.Warn("couldn't delete a finished download's parts", "dir", e.workDir(d), "err", err)
	}
	tidyParts(d)
}

// workName is a work folder's name, as Snag makes them.
var workName = regexp.MustCompile(`^[0-9a-f]{32}$`)

// sweepParts deletes unowned work folders (left by a removal that couldn't finish) from the parts
// folders in use, and those parts folders once empty.
func (e *Engine) sweepParts() {
	dirs := map[string]bool{e.settings.DownloadDir: true}
	owned := map[string]bool{}
	for _, en := range e.entries {
		dirs[en.d.Directory] = true
		if en.d.Status != StatusDone && en.d.WorkIn != "" {
			owned[filepath.Join(en.d.WorkIn, en.d.Work)] = true
		}
	}
	for dir := range dirs {
		if dir == "" {
			continue
		}
		parts := filepath.Join(dir, partsFolder)
		entries, err := os.ReadDir(parts)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			work := filepath.Join(parts, entry.Name())
			if entry.IsDir() && workName.MatchString(entry.Name()) && !owned[work] {
				_ = os.RemoveAll(work)
			}
		}
		_ = os.Remove(parts) // only if empty
	}
}
