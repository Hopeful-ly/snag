package platform

import (
	"cmp"
	"errors"
	"slices"
)

var errUnknownBrowser = errors.New("that browser isn't installed")

// Browser is an installed Chromium-family browser, which can load an unpacked extension.
type Browser struct {
	ID   string `json:"id"` // how OpenExtensionsPage knows it
	Name string `json:"name"`
}

type knownBrowser struct {
	name, page string
}

var chromium = map[string]knownBrowser{
	"chrome":   {"Chrome", "chrome://extensions"},
	"chromium": {"Chromium", "chrome://extensions"},
	"edge":     {"Edge", "edge://extensions"},
	"brave":    {"Brave", "brave://extensions"},
	"vivaldi":  {"Vivaldi", "vivaldi://extensions"},
	"opera":    {"Opera", "opera://extensions"},
}

func sortBrowsers(list []Browser) []Browser {
	slices.SortFunc(list, func(a, b Browser) int { return cmp.Compare(a.Name, b.Name) })
	return list
}
