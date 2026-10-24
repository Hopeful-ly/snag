package app

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/Roham1299/snag/internal/engine"
)

// Bytes of a video's start needed before it can be watched while downloading.
const (
	watchReadyBytes       = 2 << 20
	watchReadyStreamBytes = 1 << 20
)

var videoExtensions = []string{
	".mp4", ".m4v", ".mkv", ".webm", ".mov", ".avi", ".flv", ".wmv", ".mpg", ".mpeg", ".ogv", ".3gp", ".ts",
}

// watchState reports "ready", "soon" (with an ETA in seconds when known) or "" if d can't be
// watched while it downloads.
func watchState(d engine.Download) (state string, in *int64) {
	readyAt := int64(watchReadyBytes)
	switch d.Kind {
	case engine.KindHTTP:
		if d.Size <= 0 || d.Estimated || !slices.Contains(videoExtensions, strings.ToLower(filepath.Ext(d.Name()))) {
			return "", nil
		}
	case engine.KindStream:
		if d.Audio != "" || strings.HasPrefix(d.Label, "Audio") {
			return "", nil // audio only
		}
		if d.Clip.IsSet() {
			return "", nil // clips only play once cut, at the end
		}
		readyAt = watchReadyStreamBytes
	default:
		return "", nil
	}
	switch d.Status {
	case engine.StatusDownloading, engine.StatusQueued, engine.StatusPaused, engine.StatusPausing:
	default:
		return "", nil
	}
	ready := readyAt
	if d.Size > 0 && !d.Estimated {
		ready = min(ready, d.Size)
	}
	if d.Kind == engine.KindStream && d.ToPlay > 0 {
		// Separate audio needs its start too; the runner knows how much must arrive first.
		if d.Speed > 0 {
			seconds := (d.ToPlay + d.Speed - 1) / d.Speed
			return "soon", &seconds
		}
		return "soon", nil
	}
	if d.Playable >= ready {
		return "ready", nil
	}
	if d.Speed > 0 {
		// Every connection works on the start first, so it fills at the full download speed.
		seconds := (ready - d.Playable + d.Speed - 1) / d.Speed
		return "soon", &seconds
	}
	return "soon", nil
}
