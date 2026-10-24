// Package tools finds and fetches the programs Snag runs: yt-dlp, ffmpeg and a JavaScript runtime.
package tools

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Paths are the programs' paths; "" for one not found.
type Paths struct {
	YtDLP  string
	FFmpeg string
	Node   string
}

// Find looks in $SNAG_TOOLS, then dataTools (the self-updated copies; "" for none), then a tools
// folder next to the app, then the PATH and wellKnownDirs.
func Find(dataTools string) Paths {
	var dirs []string
	for _, dir := range []string{os.Getenv("SNAG_TOOLS"), dataTools} {
		if dir != "" {
			dirs = append(dirs, dir)
		}
	}
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(exe), "tools"))
	}
	return Paths{
		YtDLP:  find("yt-dlp", dirs),
		FFmpeg: find("ffmpeg", dirs),
		Node:   find("node", nil),
	}
}

func find(name string, dirs []string) string {
	for _, dir := range dirs {
		// LookPath also tries .exe and the like on Windows.
		if path, err := exec.LookPath(filepath.Join(dir, name)); err == nil {
			return path
		}
	}
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	for _, dir := range wellKnownDirs {
		if path, err := exec.LookPath(filepath.Join(dir, name)); err == nil {
			return path
		}
	}
	return ""
}
