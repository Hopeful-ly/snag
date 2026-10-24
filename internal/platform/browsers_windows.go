package platform

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// browserPrograms' places are relative to Program Files and the user's programs folder.
var browserPrograms = map[string]struct {
	exe    string
	places []string
}{
	"chrome":   {"chrome.exe", []string{`Google\Chrome\Application`}},
	"chromium": {"chrome.exe", []string{`Chromium\Application`}},
	"edge":     {"msedge.exe", []string{`Microsoft\Edge\Application`}},
	"brave":    {"brave.exe", []string{`BraveSoftware\Brave-Browser\Application`}},
	"vivaldi":  {"vivaldi.exe", []string{`Vivaldi\Application`}},
	"opera":    {"opera.exe", []string{`Opera`, `Programs\Opera`}},
}

// ChromiumBrowsers lists the installed Chromium-family browsers.
func ChromiumBrowsers() []Browser {
	var list []Browser
	for key, program := range browserPrograms {
		if path := findBrowser(key, program.exe, program.places); path != "" {
			list = append(list, Browser{ID: key + ":" + path, Name: chromium[key].name})
		}
	}
	return sortBrowsers(list)
}

func findBrowser(key, exe string, places []string) string {
	if key != "chromium" { // Chromium's program is called chrome.exe too: only look in its folder
		for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
			if path := appPath(root, exe); path != "" && exists(path) {
				return path
			}
		}
	}
	for _, dir := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LOCALAPPDATA")} {
		for _, place := range places {
			if path := filepath.Join(dir, place, exe); dir != "" && exists(path) {
				return path
			}
		}
	}
	return ""
}

// OpenExtensionsPage opens a browser ChromiumBrowsers listed on its extensions page.
func OpenExtensionsPage(id string) error {
	key, path, ok := strings.Cut(id, ":")
	known, isKnown := chromium[key]
	if !ok || !isKnown || !exists(path) {
		return errUnknownBrowser
	}
	return launch(path, known.page)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
