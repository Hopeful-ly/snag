package platform

import (
	"os"
	"path/filepath"
	"strings"
)

// AtLoginFlag is passed to Snag when it starts at login.
const AtLoginFlag = "--at-login"

// Autostart starts Snag at login: an XDG autostart entry on Linux, the Run key on Windows, a
// LaunchAgent on macOS. Enabled reads the system's state, so changes made there show in Snag.
type Autostart struct {
	Program string
}

// AutostartFor returns the Autostart for the running executable, and false for builds that must
// never start at login: `go run` builds and ones using $SNAG_HOME.
func AutostartFor() (Autostart, bool) {
	exe, err := os.Executable()
	if err != nil {
		return Autostart{}, false
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	tmp := os.TempDir()
	if resolved, err := filepath.EvalSymlinks(tmp); err == nil {
		tmp = resolved
	}
	temporary := strings.HasPrefix(exe, tmp+string(filepath.Separator)) ||
		strings.Contains(exe, string(filepath.Separator)+"go-build")
	return Autostart{Program: exe}, !temporary && os.Getenv("SNAG_HOME") == ""
}

// Enabled reports whether Snag starts at login.
func (a Autostart) Enabled() (bool, error) { return a.enabled() }

// Set makes Snag start at login, or not.
func (a Autostart) Set(on bool) error {
	if on {
		return a.enable()
	}
	return a.disable()
}
