package platform

import (
	"os/exec"
	"strings"
)

var browserCommands = map[string][]string{
	"chrome":   {"google-chrome-stable", "google-chrome"},
	"chromium": {"chromium", "chromium-browser"},
	"edge":     {"microsoft-edge-stable", "microsoft-edge"},
	"brave":    {"brave-browser", "brave"},
	"vivaldi":  {"vivaldi-stable", "vivaldi"},
	"opera":    {"opera"},
}

// ChromiumBrowsers lists the installed Chromium-family browsers.
func ChromiumBrowsers() []Browser {
	var list []Browser
	for key, commands := range browserCommands {
		for _, command := range commands {
			if path, err := exec.LookPath(command); err == nil {
				list = append(list, Browser{ID: key + ":" + path, Name: chromium[key].name})
				break
			}
		}
	}
	return sortBrowsers(list)
}

// OpenExtensionsPage opens a browser ChromiumBrowsers listed on its extensions page.
func OpenExtensionsPage(id string) error {
	key, path, ok := strings.Cut(id, ":")
	known, isKnown := chromium[key]
	if !ok || !isKnown {
		return errUnknownBrowser
	}
	return launch(path, known.page)
}
