package platform

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// autostartFile is in $XDG_CONFIG_HOME/autostart, per the XDG Autostart spec.
func autostartFile() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "autostart", "snag.desktop"), nil
}

func (a Autostart) enabled() (bool, error) {
	path, err := autostartFile()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// Desktops' startup settings turn an entry off by marking it, not deleting it.
	for line := range strings.SplitSeq(string(data), "\n") {
		switch strings.TrimSpace(line) {
		case "Hidden=true", "X-GNOME-Autostart-enabled=false":
			return false, nil
		}
	}
	return true, nil
}

func (a Autostart) enable() error {
	path, err := autostartFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	program := desktopQuote(a.Program)
	// TryExec: once Snag is uninstalled, the desktop skips the entry instead of failing on it.
	entry := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=Snag\n" +
		"Comment=Snag, in the tray, for downloads and the browser extension\n" +
		"Exec=" + program + " " + AtLoginFlag + "\n" +
		"TryExec=" + program + "\n" +
		"Icon=snag\n" +
		"Terminal=false\n" +
		"X-GNOME-Autostart-enabled=true\n"
	tmp := path + ".new"
	if err := os.WriteFile(tmp, []byte(entry), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (a Autostart) disable() error {
	path, err := autostartFile()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// desktopQuote quotes a path for an Exec line when needed.
func desktopQuote(path string) string {
	if !strings.ContainsAny(path, " \t\"'\\$`") {
		return path
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", "$", `\$`)
	return `"` + r.Replace(path) + `"`
}

// ownStartup lists desktops that ignore ~/.config/autostart unless something like uwsm runs it.
var ownStartup = map[string]string{
	"hyprland": "Hyprland", "sway": "Sway", "river": "river", "niri": "niri", "i3": "i3",
	"bspwm": "bspwm", "qtile": "Qtile", "dwm": "dwm", "labwc": "labwc", "dwl": "dwl",
	"herbstluftwm": "herbstluftwm", "awesome": "awesome",
}

// Manual names a desktop that ignores the autostart entry and the command to add to its own
// startup config; "" when the entry is enough.
func (a Autostart) Manual() (desktop, command string) {
	for name := range strings.SplitSeq(os.Getenv("XDG_CURRENT_DESKTOP"), ":") {
		if d, ok := ownStartup[strings.ToLower(strings.TrimSpace(name))]; ok {
			desktop = d
			break
		}
	}
	if desktop == "" || autostartRunsAnyway() {
		return "", ""
	}
	return desktop, desktopQuote(a.Program) + " " + AtLoginFlag
}

// autostartRunsAnyway reports whether systemd's xdg-desktop-autostart target (as uwsm starts it) is active.
func autostartRunsAnyway() bool {
	out, err := exec.Command("systemctl", "--user", "is-active", "xdg-desktop-autostart.target").Output()
	return err == nil && strings.TrimSpace(string(out)) == "active"
}
