package platform

import (
	"os"
	"strings"
	"testing"
)

func TestLinuxAutostartEntry(t *testing.T) {
	scratchAutostart(t)
	a := Autostart{Program: "/home/me/Snag Apps/snag"}
	if err := a.Set(true); err != nil {
		t.Fatal(err)
	}
	path, _ := autostartFile()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	entry := string(data)
	for _, want := range []string{
		`Exec="/home/me/Snag Apps/snag" --at-login`,
		`TryExec="/home/me/Snag Apps/snag"`,
		"Type=Application",
	} {
		if !strings.Contains(entry, want) {
			t.Errorf("entry lacks %q:\n%s", want, entry)
		}
	}
	// The desktop's startup settings turned it off.
	if err := os.WriteFile(path, []byte(entry+"Hidden=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if on, _ := a.Enabled(); on {
		t.Error("an entry the desktop hid counts as on")
	}
}

func TestManualOnDesktopsThatIgnoreAutostart(t *testing.T) {
	a := Autostart{Program: "/home/me/.local/lib/snag/snag"}
	t.Setenv("XDG_CURRENT_DESKTOP", "KDE")
	if d, c := a.Manual(); d != "" || c != "" {
		t.Errorf("KDE: %q %q", d, c)
	}
	t.Setenv("XDG_CURRENT_DESKTOP", "Hyprland")
	d, c := a.Manual()
	if autostartRunsAnyway() {
		t.Skip("this computer runs autostart entries on any desktop")
	}
	if d != "Hyprland" || c != "/home/me/.local/lib/snag/snag --at-login" {
		t.Errorf("Hyprland: %q %q", d, c)
	}
}
