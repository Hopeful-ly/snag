package app

import (
	"net/url"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// newLinksWindows makes the "Download links" windows, one per set of links.
func newLinksWindows(a *App) *dialogs {
	return newDialogs(a, func(id string) application.WebviewWindowOptions {
		title := "Download links"
		if svc, err := a.service(); err == nil {
			if s, err := svc.Links(id); err == nil && s.Title != "" {
				title += ": " + s.Title
			}
		}
		return application.WebviewWindowOptions{
			Name:      "links-" + id,
			Title:     title,
			Width:     720,
			Height:    700,
			MinWidth:  520,
			MinHeight: 440,
			URL:       "/?links=" + url.QueryEscape(id),
		}
	})
}

// ShowLinks opens the "Download links" window for a set of links.
func (a *App) ShowLinks(id string) {
	a.links.show(id)
}
