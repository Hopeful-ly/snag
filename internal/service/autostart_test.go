package service_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Roham1299/snag/internal/platform"
)

// Turned off after the first start, autostart stays off across restarts.
func TestStartsAtLoginOnceByDefault(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("keeps the entry in a folder of the test's own on Linux only")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	entry := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "autostart", "snag.desktop")
	a := platform.Autostart{Program: "/opt/snag/snag"}

	app := startApp(t, "")
	if on, available, _ := app.StartAtLogin(); on || available {
		t.Fatalf("before UseAutostart: on %v, available %v", on, available)
	}
	if err := app.SetStartAtLogin(true); err == nil {
		t.Error("turned on starting at login without knowing how")
	}
	app.UseAutostart(a)
	if _, err := os.Stat(entry); err != nil {
		t.Fatalf("no entry after the first start: %v", err)
	}
	if on, available, err := app.StartAtLogin(); !on || !available || err != nil {
		t.Fatalf("StartAtLogin = %v, %v, %v", on, available, err)
	}
	if !app.Config().StartAtLoginSet {
		t.Error("the settings don't say it was set")
	}

	if err := app.SetStartAtLogin(false); err != nil {
		t.Fatal(err)
	}
	app.UseAutostart(a) // the next start
	if _, err := os.Stat(entry); !os.IsNotExist(err) {
		t.Fatalf("turned on again after the user turned it off: %v", err)
	}

	data, err := os.ReadFile(app.paths.Config)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(data, &saved); err != nil || saved["startAtLoginSet"] != true {
		t.Errorf("settings file: %s, %v", data, err)
	}
}
