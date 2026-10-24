package app

import (
	"errors"

	"github.com/Roham1299/snag/internal/version"
)

// SystemService exposes app-level helpers to the web UI.
type SystemService struct {
	app *App
}

// Version returns the running version of Snag.
func (s *SystemService) Version() string {
	return version.String()
}

// ClipboardText returns the clipboard text, which the web view may not be allowed to read.
func (s *SystemService) ClipboardText() string {
	text, _ := s.app.wails.Clipboard.Text()
	return text
}

// CopyText puts text on the clipboard.
func (s *SystemService) CopyText(text string) error {
	if !s.app.wails.Clipboard.SetText(text) {
		return errors.New("couldn't copy it")
	}
	return nil
}

const (
	authorName = "Roham1299"
	authorPage = "https://github.com/Roham1299"
	// The website sends this to the latest signed Firefox add-on, which Firefox offers to install.
	firefoxAddon = "https://snagdl.com/firefox"
)

// Author is who made Snag, with their page.
type Author struct {
	Name string `json:"name"`
	Page string `json:"page"`
}

// Author returns who made Snag.
func (s *SystemService) Author() Author {
	return Author{Name: authorName, Page: authorPage}
}

// OpenAuthorPage opens the author's page. It takes no URL so the window can't open arbitrary
// addresses.
func (s *SystemService) OpenAuthorPage() error {
	return s.app.wails.Browser.OpenURL(authorPage)
}

// OpenFirefoxAddon opens the page that installs Snag's Firefox add-on.
func (s *SystemService) OpenFirefoxAddon() error {
	return s.app.wails.Browser.OpenURL(firefoxAddon)
}
