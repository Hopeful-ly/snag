package app

import (
	"os"

	"github.com/Roham1299/snag/internal/nativehost"
	"github.com/Roham1299/snag/internal/platform"
)

// SetupService helps connect the browser extension, which is loaded unpacked from Snag's folder
// and talks to Snag through its helper.
type SetupService struct {
	app *App
}

// Setup is the browser connection status.
type Setup struct {
	ExtensionDir string `json:"extensionDir"`
	// Connected covers this run only; ConnectedBefore covers any run.
	Connected       bool `json:"connected"`
	ConnectedBefore bool `json:"connectedBefore"`
}

// Status returns the browser connection status.
func (s *SetupService) Status() (Setup, error) {
	svc, err := s.app.service()
	if err != nil {
		return Setup{}, err
	}
	return Setup{
		ExtensionDir:    svc.ExtensionDir(),
		Connected:       !svc.LastContact().IsZero(),
		ConnectedBefore: svc.Config().BrowserConnected,
	}, nil
}

// ShowExtensionFolder selects the extension's folder in the file manager, for dragging onto a
// browser's extensions page.
func (s *SetupService) ShowExtensionFolder() error {
	svc, err := s.app.service()
	if err != nil {
		return err
	}
	return platform.Reveal(svc.ExtensionDir())
}

// Browser is an installed Chromium-family browser.
type Browser = platform.Browser

// Browsers lists the installed Chromium-family browsers.
func (s *SetupService) Browsers() []Browser {
	return platform.ChromiumBrowsers()
}

// OpenExtensionsPage opens a browser (from Browsers) on its extensions page.
func (s *SetupService) OpenExtensionsPage(id string) error {
	return platform.OpenExtensionsPage(id)
}

// RegisterBrowsers re-registers Snag's helper with the browsers and returns their names.
func (s *SetupService) RegisterBrowsers() ([]string, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return nativehost.RegisterNextTo(exe)
}
