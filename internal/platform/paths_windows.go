package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func configDir() (string, error) {
	dir, err := os.UserConfigDir() // %AppData%, which roams with the user
	return filepath.Join(dir, "Snag"), err
}

// The download list and unfinished downloads stay on this machine (%LocalAppData%).
func dataDir(string) (string, error) {
	dir := os.Getenv("LOCALAPPDATA")
	if dir == "" {
		return "", errors.New("%LocalAppData% is not set")
	}
	return filepath.Join(dir, "Snag"), nil
}

// Named pipes live in one namespace for the whole machine, so the user's name keeps them apart.
func socketPath(string) string {
	return `\\.\pipe\snag-` + strings.ToLower(os.Getenv("USERNAME"))
}

// socketIn is the pipe for an app that keeps everything in dir ($SNAG_HOME): named after the
// folder, as pipes can't live in one.
func socketIn(dir string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(dir))))
	return `\\.\pipe\snag-` + hex.EncodeToString(sum[:8])
}

func downloadsDir(home string) string {
	return filepath.Join(home, "Downloads")
}
