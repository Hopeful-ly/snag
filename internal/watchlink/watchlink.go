// Package watchlink links a download's final name to its growing file so it can be played early.
// Hard links need no special permissions on any OS; symlinks are the fallback across drives
// (Unix, or Windows in developer mode).
package watchlink

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// hardLink is swapped out by tests to try the fallback.
var hardLink = os.Link

// Make links path to target, never replacing another file, and reports whether path shows target.
func Make(path, target string) bool {
	if Shows(path, target) {
		return true
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return false // a real file that isn't ours
		}
		_ = os.Remove(path) // a link left by an earlier run
	}
	if hardLink(target, path) == nil {
		return true
	}
	return os.Symlink(target, path) == nil
}

// Shows reports whether path links to target by either kind of link.
func Shows(path, target string) bool {
	a, err := os.Stat(path)
	if err != nil {
		return false
	}
	b, err := os.Stat(target)
	return err == nil && os.SameFile(a, b)
}

// IsHard reports whether path is a hard link to target, so only target's name has to go.
func IsHard(path, target string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular() && Shows(path, target)
}

// Finish moves file to path. If linked, the earlier link is replaced even if stale (ffmpeg may
// have rewritten the file); a hard link to file is already the file in place and is kept.
func Finish(path, file string, linked bool) error {
	if IsHard(path, file) {
		return nil
	}
	if linked {
		if info, err := os.Lstat(path); err == nil && (info.Mode()&os.ModeSymlink != 0 || info.Mode().IsRegular()) {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}
	return move(file, path)
}

// move copies across drives.
func move(from, to string) error {
	if err := os.Rename(from, to); err == nil {
		return nil
	}
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(to)
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	src.Close()
	return os.Remove(from)
}

// RemoveInto removes path if it links to a file inside dir, for when a work folder is deleted.
func RemoveInto(path, dir string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if target, err := os.Readlink(path); err == nil && within(target, dir) {
			return os.Remove(path)
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.Type().IsRegular() && Shows(path, filepath.Join(dir, e.Name())) {
			return os.Remove(path)
		}
	}
	return nil
}

func within(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
