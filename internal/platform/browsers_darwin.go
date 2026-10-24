package platform

import (
	"os"
	"path/filepath"
	"strings"
)

var browserApps = map[string]string{
	"chrome": "Google Chrome.app", "chromium": "Chromium.app", "edge": "Microsoft Edge.app",
	"brave": "Brave Browser.app", "vivaldi": "Vivaldi.app", "opera": "Opera.app",
}

// ChromiumBrowsers lists the installed Chromium-family browsers.
func ChromiumBrowsers() []Browser {
	home, _ := os.UserHomeDir()
	var list []Browser
	for key, app := range browserApps {
		for _, dir := range []string{"/Applications", filepath.Join(home, "Applications")} {
			path := filepath.Join(dir, app)
			if _, err := os.Stat(path); err == nil {
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
	return launch("open", "-a", path, known.page)
}
