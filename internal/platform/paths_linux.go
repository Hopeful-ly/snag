package platform

import (
	"os"
	"path/filepath"
)

func configDir() (string, error) {
	dir, err := os.UserConfigDir() // $XDG_CONFIG_HOME or ~/.config
	return filepath.Join(dir, "snag"), err
}

func dataDir(home string) (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "snag"), nil
	}
	return filepath.Join(home, ".local", "share", "snag"), nil
}

// The runtime folder is private to the user and emptied at logout, as a socket's place should be.
func socketPath(data string) string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "snag", "app.sock")
	}
	return filepath.Join(data, "app.sock")
}

// socketIn is the socket for an app that keeps everything in dir ($SNAG_HOME).
func socketIn(dir string) string {
	return filepath.Join(dir, "app.sock")
}

func downloadsDir(home string) string {
	if dir := os.Getenv("XDG_DOWNLOAD_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(home, "Downloads")
}
