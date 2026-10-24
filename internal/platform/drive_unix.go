//go:build !windows

package platform

import (
	"os"
	"path/filepath"
	"syscall"
)

// SameDrive reports whether a and b are on one file system, so renames and hard links work. If unsure, it says yes.
func SameDrive(a, b string) bool {
	da, okA := device(existing(a))
	db, okB := device(existing(b))
	return !okA || !okB || da == db
}

func device(path string) (uint64, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return uint64(st.Dev), true //nolint:unconvert // Dev's type differs between systems
}

// Hide is a no-op: a leading dot already hides a folder here.
func Hide(string) error { return nil }

// existing returns path or its nearest existing ancestor, since a download folder may not exist yet.
func existing(path string) string {
	path = filepath.Clean(path)
	for {
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return path
		}
		path = parent
	}
}
