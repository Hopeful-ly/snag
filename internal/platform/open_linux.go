package platform

import (
	"net/url"
	"os/exec"
	"path/filepath"
)

// Open opens a file with its usual program.
func Open(path string) error {
	return launch("xdg-open", path)
}

// Reveal shows a file in the file manager, selected where the file manager supports that.
func Reveal(path string) error {
	err := exec.Command("dbus-send", "--session", "--dest=org.freedesktop.FileManager1", "--type=method_call",
		"/org/freedesktop/FileManager1", "org.freedesktop.FileManager1.ShowItems",
		"array:string:"+(&url.URL{Scheme: "file", Path: path}).String(), "string:").Run()
	if err != nil {
		return launch("xdg-open", filepath.Dir(path))
	}
	return nil
}
