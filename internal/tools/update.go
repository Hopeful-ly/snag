package tools

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Roham1299/snag/internal/platform"
)

// PrepareYtDLP copies the bundled yt-dlp into dataTools, where it can be updated (install folders
// are often read-only), again whenever an app update brings a newer one. It returns the copy's
// path, or "" without a bundled yt-dlp.
func PrepareYtDLP(dataTools string) string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	name := Host().exe("yt-dlp")
	bundled := filepath.Join(filepath.Dir(exe), "tools", name)
	from, err := os.Stat(bundled)
	if err != nil {
		return ""
	}
	copied := filepath.Join(dataTools, name)
	if to, err := os.Stat(copied); err == nil && !from.ModTime().After(to.ModTime()) {
		return copied
	}
	if err := copyFile(bundled, copied); err != nil {
		slog.Warn("couldn't set up yt-dlp for updates", "err", err)
		return ""
	}
	return copied
}

// UpdateYtDLP runs yt-dlp's updater on path to version to, which may be older than the current
// one (to roll back a broken release); empty means the latest stable, even from a nightly.
func UpdateYtDLP(ctx context.Context, path, to string) {
	if path == "" {
		return
	}
	if to == "" {
		to = "stable"
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--update-to", to, "--ignore-config")
	platform.StartGroup(cmd) // no console window on Windows
	out, err := cmd.CombinedOutput()
	result := strings.TrimSpace(lastLine(string(out)))
	if err != nil {
		slog.Warn("yt-dlp couldn't update itself", "err", err, "said", result)
		return
	}
	slog.Info("yt-dlp update", "result", result)
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}

func copyFile(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	data, err := io.ReadAll(src)
	if err != nil {
		return err
	}
	return writeExecutable(to, data)
}
