package platform

import (
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// macOS starts the agents in ~/Library/LaunchAgents at login.
const launchAgentLabel = "app.snag.desktop"

func launchAgentFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist"), nil
}

func (a Autostart) enabled() (bool, error) {
	path, err := launchAgentFile()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (a Autostart) enable() error {
	path, err := launchAgentFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var program strings.Builder
	if err := xml.EscapeText(&program, []byte(a.Program)); err != nil {
		return err
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + launchAgentLabel + `</string>
	<key>ProgramArguments</key>
	<array>
		<string>` + program.String() + `</string>
		<string>` + AtLoginFlag + `</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>ProcessType</key>
	<string>Interactive</string>
</dict>
</plist>
`
	tmp := path + ".new"
	if err := os.WriteFile(tmp, []byte(plist), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (a Autostart) disable() error {
	path, err := launchAgentFile()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Manual is "" on macOS: LaunchAgents are always read.
func (a Autostart) Manual() (desktop, command string) { return "", "" }
