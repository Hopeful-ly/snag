package platform

import (
	"os"
	"path/filepath"
)

func configDir() (string, error) {
	dir, err := os.UserConfigDir() // ~/Library/Application Support
	return filepath.Join(dir, "Snag"), err
}

func dataDir(home string) (string, error) {
	return filepath.Join(home, "Library", "Application Support", "Snag"), nil
}

func socketPath(data string) string {
	return filepath.Join(data, "app.sock")
}

// socketIn is the socket for an app that keeps everything in dir ($SNAG_HOME).
func socketIn(dir string) string {
	return filepath.Join(dir, "app.sock")
}

func downloadsDir(home string) string {
	return filepath.Join(home, "Downloads")
}
