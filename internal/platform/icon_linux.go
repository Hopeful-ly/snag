package platform

import (
	"bytes"
	"os"
	"path/filepath"
)

const linuxAppID = "app.snag.desktop"

// InstallAppIcon installs the icon into the icon theme, since GTK4 can't set a window icon from bytes.
func InstallAppIcon(png, svg []byte) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	base := filepath.Join(home, ".local/share/icons/hicolor")
	writeIfNew(filepath.Join(base, "256x256/apps", linuxAppID+".png"), png)
	writeIfNew(filepath.Join(base, "256x256/apps", "snag.png"), png)
	if len(svg) > 0 {
		writeIfNew(filepath.Join(base, "scalable/apps", linuxAppID+".svg"), svg)
		writeIfNew(filepath.Join(base, "scalable/apps", "snag.svg"), svg)
	}
}

func writeIfNew(path string, data []byte) {
	if have, err := os.ReadFile(path); err == nil && bytes.Equal(have, data) {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}
