package update

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// app is the Snag.app this copy runs from, or "" when it isn't in one it can replace.
var app = sync.OnceValue(func() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return ""
	}
	bundle := filepath.Dir(filepath.Dir(filepath.Dir(exe))) // Snag.app/Contents/MacOS/snag
	if !strings.HasSuffix(bundle, ".app") {
		return ""
	}
	probe, err := os.CreateTemp(filepath.Dir(bundle), ".snag-*")
	if err != nil {
		return "" // a folder the user can't write to, such as /Applications for a standard user
	}
	probe.Close()
	os.Remove(probe.Name())
	return bundle
})

func kind() string {
	if app() == "" {
		return ""
	}
	return "macos"
}

// install puts the disk image's Snag.app in place of this one, then starts it once this one has
// quit. The old app is removed after that.
func install(file string) error {
	bundle := app()
	if bundle == "" {
		return errors.New("this copy of Snag can't update itself")
	}
	mount, err := os.MkdirTemp("", "snag-update")
	if err != nil {
		return err
	}
	defer os.Remove(mount)
	if out, err := exec.Command("hdiutil", "attach", "-nobrowse", "-readonly", "-noautoopen", "-mountpoint", mount, file).CombinedOutput(); err != nil {
		return fmt.Errorf("couldn't open the update: %w: %s", err, strings.TrimSpace(string(out)))
	}
	defer exec.Command("hdiutil", "detach", mount, "-force").Run() //nolint:errcheck // a leftover mount goes at logout

	parent := filepath.Dir(bundle)
	fresh, old := filepath.Join(parent, ".Snag-update.app"), filepath.Join(parent, ".Snag-old.app")
	_ = os.RemoveAll(fresh)
	_ = os.RemoveAll(old)
	if out, err := exec.Command("ditto", filepath.Join(mount, "Snag.app"), fresh).CombinedOutput(); err != nil {
		_ = os.RemoveAll(fresh)
		return fmt.Errorf("couldn't copy the update: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if err := os.Rename(bundle, old); err != nil {
		_ = os.RemoveAll(fresh)
		return err
	}
	if err := os.Rename(fresh, bundle); err != nil {
		_ = os.Rename(old, bundle)
		return err
	}
	return afterExit(`rm -rf "$2"; exec open "$1"`, bundle, old)
}
