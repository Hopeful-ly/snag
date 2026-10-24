package platform

import (
	"path/filepath"
	"strings"
	"syscall"
)

// SameDrive reports whether a and b are on one drive, so renames and hard links work.
func SameDrive(a, b string) bool {
	va := filepath.VolumeName(absolute(a))
	vb := filepath.VolumeName(absolute(b))
	return strings.EqualFold(va, vb)
}

func absolute(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// Hide hides a folder from Explorer, which ignores a leading dot.
func Hide(path string) error {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attrs, err := syscall.GetFileAttributes(name)
	if err != nil {
		return err
	}
	return syscall.SetFileAttributes(name, attrs|syscall.FILE_ATTRIBUTE_HIDDEN)
}
