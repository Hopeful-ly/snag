// Package platform holds what differs between Windows, macOS and Linux.
package platform

import (
	"os"
	"path/filepath"
)

// Paths are where Snag keeps its files.
type Paths struct {
	Config string
	// State is the saved download list.
	State string
	// Work holds one folder per unfinished download.
	Work      string
	Downloads string
	// Tools holds the self-updated yt-dlp.
	Tools string
	// Extension holds the unpacked browser extension, kept current by the app.
	Extension string
	// Socket is a Unix socket or Windows named pipe; $SNAG_SOCKET overrides it.
	Socket string
}

// DefaultPaths returns the usual places for this OS and user, or everything under $SNAG_HOME if set.
func DefaultPaths() (Paths, error) {
	if home := os.Getenv("SNAG_HOME"); home != "" {
		return pathsIn(home), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	config, err := configDir()
	if err != nil {
		return Paths{}, err
	}
	data, err := dataDir(home)
	if err != nil {
		return Paths{}, err
	}
	socket := os.Getenv("SNAG_SOCKET")
	if socket == "" {
		socket = socketPath(data)
	}
	return Paths{
		Config:    filepath.Join(config, "settings.json"),
		State:     filepath.Join(data, "list.json"),
		Work:      filepath.Join(data, "work"),
		Extension: filepath.Join(data, "extension"),
		Tools:     filepath.Join(data, "tools"),
		Downloads: downloadsDir(home),
		Socket:    socket,
	}, nil
}

func pathsIn(dir string) Paths {
	socket := os.Getenv("SNAG_SOCKET")
	if socket == "" {
		socket = socketIn(dir)
	}
	return Paths{
		Config:    filepath.Join(dir, "settings.json"),
		State:     filepath.Join(dir, "list.json"),
		Work:      filepath.Join(dir, "work"),
		Extension: filepath.Join(dir, "extension"),
		Tools:     filepath.Join(dir, "tools"),
		Downloads: filepath.Join(dir, "downloads"),
		Socket:    socket,
	}
}
