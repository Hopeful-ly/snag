// Command snag-host is the native messaging helper browsers start for the Snag extension.
// Standard output belongs to the browser: nothing but messages may be written to it.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Roham1299/snag/internal/nativemsg"
	"github.com/Roham1299/snag/internal/platform"
)

func main() {
	paths, err := platform.DefaultPaths()
	if err != nil {
		os.Exit(1)
	}
	relay := &nativemsg.Relay{Socket: paths.Socket, StartApp: startApp, FromBrowser: os.Stdin, ToBrowser: os.Stdout}
	if err := relay.Run(); err != nil {
		os.Exit(1)
	}
}

// startApp starts the Snag app next to this executable, in the background.
func startApp() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	app, err := exec.LookPath(filepath.Join(filepath.Dir(exe), "snag"))
	if err != nil {
		return err
	}
	cmd := exec.Command(app, "--background")
	cmd.Env = withoutBrowserHacks(os.Environ())
	platform.StartDetached(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// withoutBrowserHacks drops browser-set environment the app's children must not inherit
// (Vivaldi preloads a stripped ffmpeg library, which breaks the real ffmpeg).
func withoutBrowserHacks(env []string) []string {
	var out []string
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if name == "LD_PRELOAD" || name == "LD_LIBRARY_PATH" || strings.HasPrefix(name, "CHROME_") {
			continue
		}
		out = append(out, kv)
	}
	return out
}
