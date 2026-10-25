// Package update installs a newer Snag from an installer named in the signed feed: it downloads
// the installer, checks it against the feed's SHA-256, installs it and starts the new Snag once
// this one has quit.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/Roham1299/snag/internal/feed"
)

// Installers are hundreds of MB at most; this stops a wrong file from filling the disk.
const maxSize = 1 << 30

var client = &http.Client{Timeout: 30 * time.Minute}

// Kind is the kind of installer this copy of Snag updates from ("windows-amd64", "deb-arm64"
// and so on, as feed.FileKind names them), or "" when it can't update itself: a development
// build, or an install that isn't one of Snag's packages.
func Kind() string {
	return kind()
}

// For returns the release's installer for this install.
func For(r *feed.Release) (feed.File, bool) {
	k := Kind()
	if r == nil || k == "" {
		return feed.File{}, false
	}
	f, ok := r.Files[k]
	return f, ok
}

// Download fetches f into dir unless it's there already, and returns its path once its SHA-256
// matches. Other files in dir, older installers, are removed.
func Download(ctx context.Context, f feed.File, dir string) (string, error) {
	u, err := url.Parse(f.URL)
	if err != nil {
		return "", err
	}
	name := path.Base(u.Path)
	if name == "." || name == "/" {
		return "", fmt.Errorf("%s names no file", f.URL)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	clean(dir, name)
	dst := filepath.Join(dir, name)
	if sum, err := hashFile(dst); err == nil && sum == f.SHA256 {
		return dst, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", f.URL, resp.Status)
	}
	tmp, err := os.CreateTemp(dir, name+".*.part")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(resp.Body, maxSize+1))
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	if n > maxSize {
		return "", errors.New("the installer is too large")
	}
	if sum := hex.EncodeToString(hash.Sum(nil)); sum != f.SHA256 {
		return "", fmt.Errorf("the installer's SHA-256 is %s, not the feed's %s", sum, f.SHA256)
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return "", err
	}
	return dst, os.Rename(tmp.Name(), dst)
}

// Install installs the installer at file, checked again against sha256 since it was
// downloaded, then arranges for the new Snag to start once this process exits. The caller
// quits next.
func Install(file, sha256 string) error {
	if sum, err := hashFile(file); err != nil || sum != sha256 {
		return errors.New("the downloaded update has changed since it was checked")
	}
	return install(file)
}

func hashFile(name string) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func clean(dir, keep string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Name() != keep {
			_ = os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
}
