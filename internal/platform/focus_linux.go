package platform

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// BringForward focuses the window with pid and title. Tiling compositors (Hyprland, Sway) ignore a
// window's own focus request, so they are asked directly; elsewhere it does nothing.
func BringForward(pid int, title string) {
	switch {
	case os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "":
		address := hyprlandWindow(pid, title)
		if address == "" {
			return
		}
		target := "address:" + address
		// Hyprland's classic command, or the form of its newer Lua-configured releases.
		if !hyprctl("dispatch", "focuswindow", target) {
			hyprctl("dispatch", fmt.Sprintf(`hl.dsp.focus({ window = "%s" })`, target))
		}
	case os.Getenv("SWAYSOCK") != "":
		pattern := strings.ReplaceAll(regexp.QuoteMeta(title), `"`, `\"`)
		_ = exec.Command("swaymsg", fmt.Sprintf(`[pid=%d title="^%s$"] focus`, pid, pattern)).Run()
	}
}

// hyprlandWindow returns the address of the window with this pid and title, or "".
func hyprlandWindow(pid int, title string) string {
	out, err := exec.Command("hyprctl", "clients", "-j").Output()
	if err != nil {
		return ""
	}
	var clients []struct {
		Address string `json:"address"`
		Pid     int    `json:"pid"`
		Title   string `json:"title"`
	}
	if json.Unmarshal(out, &clients) != nil {
		return ""
	}
	for _, c := range clients {
		if c.Pid == pid && c.Title == title {
			return c.Address
		}
	}
	return ""
}

func hyprctl(args ...string) bool {
	out, err := exec.Command("hyprctl", args...).Output()
	return err == nil && strings.TrimSpace(string(out)) == "ok"
}
