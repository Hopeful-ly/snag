package platform

import (
	"path/filepath"
	"runtime"
	"testing"
)

// scratchAutostart redirects the entry so running the tests never adds a real login item.
func scratchAutostart(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	switch runtime.GOOS {
	case "linux":
		t.Setenv("XDG_CONFIG_HOME", dir)
	case "darwin":
		t.Setenv("HOME", dir)
	case "windows":
		useTestAutostartValue(t)
	}
}

func TestAutostartOnAndOff(t *testing.T) {
	scratchAutostart(t)
	a := Autostart{Program: filepath.Join(t.TempDir(), "Snag Folder", "snag")}
	if on, err := a.Enabled(); err != nil || on {
		t.Fatalf("Enabled before = %v, %v", on, err)
	}
	if err := a.Set(true); err != nil {
		t.Fatal(err)
	}
	if on, err := a.Enabled(); err != nil || !on {
		t.Fatalf("Enabled after turning on = %v, %v", on, err)
	}
	if err := a.Set(true); err != nil { // idempotent
		t.Fatal(err)
	}
	if err := a.Set(false); err != nil {
		t.Fatal(err)
	}
	if on, err := a.Enabled(); err != nil || on {
		t.Fatalf("Enabled after turning off = %v, %v", on, err)
	}
	if err := a.Set(false); err != nil { // idempotent
		t.Fatal(err)
	}
}

func TestAutostartForADevelopmentBuild(t *testing.T) {
	// The test binary lives in Go's temporary folder, as `go run` builds do.
	if _, ok := AutostartFor(); ok {
		t.Error("a build in the temporary folder may start at login")
	}
}
