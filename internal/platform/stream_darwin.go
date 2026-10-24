package platform

import (
	"os"
	"path/filepath"
)

var macPlayers = []struct{ app, name string }{{"VLC.app", "VLC"}, {"IINA.app", "IINA"}, {"mpv.app", "mpv"}}

// PlayStream always returns ErrNoStreamPlayer on macOS; callers use StreamPlayers instead.
func PlayStream(address, name string) error {
	return ErrNoStreamPlayer
}

// PlayStreamWith opens address in player id (an .app from StreamPlayers).
func PlayStreamWith(id, address string) error {
	return launch("open", "-a", id, address)
}

// StreamPlayers lists the installed players that play streams.
func StreamPlayers(string) []Player {
	home, _ := os.UserHomeDir()
	var players []Player
	for _, p := range macPlayers {
		for _, dir := range []string{"/Applications", filepath.Join(home, "Applications")} {
			path := filepath.Join(dir, p.app)
			if _, err := os.Stat(path); err == nil {
				players = append(players, Player{ID: path, Name: p.name})
				break
			}
		}
	}
	return players
}
