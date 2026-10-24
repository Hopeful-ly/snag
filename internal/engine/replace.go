package engine

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/Roham1299/snag/internal/naming"
)

// Clash is a name already taken in a folder, for the Save dialog to ask about.
type Clash struct {
	Existing string `json:"existing"`
	// InUse means another download in Snag is saving to that name, so it can't be replaced.
	InUse bool `json:"inUse"`
	// KeepBoth is the name the download gets next to it.
	KeepBoth string `json:"keepBoth"`
}

// The containers yt-dlp may give a stream, which is named before its container is known.
var streamContainers = []string{".mp4", ".mkv", ".webm", ".m4a", ".mp3", ".opus", ".ogg"}

// ErrNameInUse is the answer to replacing a name another download is saving to.
var ErrNameInUse = errors.New("another download in Snag is saving to that name; choose another")

// Clash reports whether downloading id into dir as filename ("" for the name it has) would meet a
// file that is there already; nil if the name is free.
func (e *Engine) Clash(id, dir, filename string) (*Clash, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	en := e.find(id)
	if en == nil {
		return nil, ErrNotFound
	}
	paths, inUse, keep := e.clashes(en, dir, filename)
	if len(paths) == 0 {
		return nil, nil
	}
	return &Clash{Existing: filepath.Base(paths[0]), InUse: inUse, KeepBoth: keep}, nil
}

// clashes returns the paths a download named filename in dir would meet, whether another
// download holds one of them, and the name it would get beside them. The lock must be held.
func (e *Engine) clashes(en *entry, dir, filename string) (paths []string, inUse bool, keepBoth string) {
	name := naming.Safe(filename, "")
	if name == "" {
		name = naming.Safe(en.d.Name(), "download")
	}
	candidates := []string{name}
	if en.d.Kind == KindStream {
		stem := streamExtension.ReplaceAllString(name, "")
		candidates = candidates[:0]
		for _, ext := range streamContainers {
			candidates = append(candidates, stem+ext)
		}
	}
	taken := func(path string) bool { return path != en.d.Path && e.pathInUse(path) }
	for _, c := range candidates {
		path := filepath.Join(dir, c)
		if taken(path) {
			paths, inUse = append(paths, path), true
		} else if _, err := os.Lstat(path); err == nil && path != en.d.Path {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return nil, false, ""
	}
	// Beside the files that are there: the first free "name (n)", in every container for a stream.
	unavailable := func(path string) bool {
		variants := []string{path}
		if en.d.Kind == KindStream {
			stem := streamExtension.ReplaceAllString(path, "")
			variants = variants[:0]
			for _, ext := range streamContainers {
				variants = append(variants, stem+ext)
			}
		}
		for _, v := range variants {
			if _, err := os.Lstat(v); (err == nil && v != en.d.Path) || taken(v) {
				return true
			}
		}
		return false
	}
	keep := naming.UniquePath(dir, filepath.Base(paths[0]), unavailable)
	keepBoth = filepath.Base(keep)
	if en.d.Kind == KindStream {
		keepBoth = streamExtension.ReplaceAllString(keepBoth, "")
	}
	return paths, inUse, keepBoth
}

// setAside is where a replaced file waits until its replacement is finished.
func setAside(path string) string {
	return filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".snag-replaced")
}

// setAsideReplaced renames the files d replaces to hidden names beside them, before it claims its
// path, so they are never deleted before the replacement is finished (dropReplaced) and come
// back if it is removed (restoreReplaced).
func (e *Engine) setAsideReplaced(d *Download) {
	for _, path := range d.Replace {
		if info, err := os.Lstat(path); err != nil || info.IsDir() {
			continue
		}
		if err := os.Rename(path, setAside(path)); err != nil {
			e.log.Warn("couldn't set a replaced file aside", "path", path, "err", err)
		}
	}
}

// dropReplaced deletes the files d replaced, now that it is finished.
func (e *Engine) dropReplaced(d *Download) {
	for _, path := range d.Replace {
		if err := os.Remove(setAside(path)); err != nil && !errors.Is(err, os.ErrNotExist) {
			e.log.Warn("couldn't delete a replaced file", "path", path, "err", err)
		}
	}
	d.Replace = nil
}

// restoreReplaced puts back the files d would have replaced, as it was removed unfinished.
func (e *Engine) restoreReplaced(d Download) {
	for _, path := range d.Replace {
		aside := setAside(path)
		if _, err := os.Lstat(aside); err != nil {
			continue
		}
		if _, err := os.Lstat(path); err == nil {
			e.log.Warn("couldn't put a replaced file back: its name is taken", "path", path, "kept", aside)
			continue
		}
		if err := os.Rename(aside, path); err != nil {
			e.log.Warn("couldn't put a replaced file back", "path", path, "err", err)
		}
	}
}
