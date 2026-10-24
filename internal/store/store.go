// Package store keeps values in JSON files. Writes are atomic (a crash leaves the old file or the
// new one, never half of each) and the files are readable only by the current user.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrCorrupt means a file wasn't valid JSON; it has been moved aside rather than overwritten.
var ErrCorrupt = errors.New("corrupt file")

// File is a value of type T kept in a JSON file.
type File[T any] struct {
	path string
	mu   sync.Mutex
}

// NewFile returns the store for path. Nothing is read or created until used.
func NewFile[T any](path string) *File[T] {
	return &File[T]{path: path}
}

// Path returns the file's path.
func (f *File[T]) Path() string {
	return f.path
}

// Load reads the file into v. A missing file leaves v as it is, so v can hold defaults.
func (f *File[T]) Load(v *T) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		aside := fmt.Sprintf("%s.broken-%s", f.path, time.Now().Format("20060102-150405"))
		if renameErr := os.Rename(f.path, aside); renameErr != nil {
			return fmt.Errorf("%w: %s: %w (and couldn't move it aside: %w)", ErrCorrupt, f.path, err, renameErr)
		}
		return fmt.Errorf("%w: %s: %w (moved to %s)", ErrCorrupt, f.path, err, aside)
	}
	return nil
}

// Save atomically replaces the file's contents with v.
func (f *File[T]) Save(v T) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return writeAtomic(f.path, data)
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// CreateTemp makes the file readable only by its owner.
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once renamed

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
