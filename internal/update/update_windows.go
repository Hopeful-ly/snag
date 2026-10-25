package update

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Roham1299/snag/internal/platform"
)

// kind is the installer's when Snag runs from where it installed itself (it leaves its
// uninstaller next to snag.exe).
func kind() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(exe), "uninstall.exe")); err != nil {
		return ""
	}
	return "windows-" + runtime.GOARCH
}

// install runs the installer quietly once this process has exited, since Windows doesn't let
// it replace a running snag.exe, then starts the new Snag.
func install(file string) error {
	if kind() == "" {
		return errors.New("this copy of Snag can't update itself")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	script := fmt.Sprintf(
		"Wait-Process -Id %d -Timeout 60 -ErrorAction SilentlyContinue; "+
			"Start-Process -Wait -FilePath %s -ArgumentList '/S'; "+
			"Start-Process -FilePath %s",
		os.Getpid(), quote(file), quote(exe))
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	platform.StartDetached(cmd)
	return cmd.Start()
}

// quote makes s a PowerShell string literal.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
