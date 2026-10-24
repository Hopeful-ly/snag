// Package cover embeds album art in song files.
package cover

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Roham1299/snag/internal/platform"
)

const maxPicture = 10 << 20

// ErrNotASong means the file isn't an MP3 or M4A.
var ErrNotASong = errors.New("not a song file Snag can put a cover in")

// Fetch downloads the picture at address into dir and returns its path.
func Fetch(ctx context.Context, client *http.Client, address, dir string) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching the cover: %s", resp.Status)
	}
	ext := ".jpg"
	if strings.Contains(resp.Header.Get("Content-Type"), "png") {
		ext = ".png"
	}
	path := filepath.Join(dir, "snag-cover"+ext)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(f, io.LimitReader(resp.Body, maxPicture))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

// Embed makes picture (JPEG or PNG) the cover of the MP3 or M4A at path, in place, replacing any
// existing cover. ffmpeg "" means the one on PATH.
func Embed(ctx context.Context, ffmpeg, path, picture string) error {
	ext := strings.ToLower(filepath.Ext(path))
	args := []string{"-v", "error", "-y", "-i", path, "-i", picture,
		"-map", "0:a", "-map", "1:0", "-c", "copy", "-map_metadata", "0",
		"-disposition:v:0", "attached_pic",
		"-metadata:s:v", "title=Album cover", "-metadata:s:v", "comment=Cover (front)"}
	switch ext {
	case ".mp3":
		args = append(args, "-id3v2_version", "3")
	case ".m4a":
	default:
		return ErrNotASong
	}
	out := strings.TrimSuffix(path, filepath.Ext(path)) + ".cover" + ext
	args = append(args, out)
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	platform.StartGroup(cmd)
	if msg, err := cmd.CombinedOutput(); err != nil {
		os.Remove(out)
		return fmt.Errorf("putting the cover in: %w: %s", err, strings.TrimSpace(string(msg)))
	}
	return os.Rename(out, path)
}
