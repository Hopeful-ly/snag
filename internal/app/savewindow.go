package app

import (
	"net/url"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/Roham1299/snag/internal/engine"
)

// saveWindows are the "Save download" windows, one per pending download. Closing one leaves the
// download pending in the list.
type saveWindows struct {
	*dialogs
}

func newSaveWindows(a *App) *saveWindows {
	return &saveWindows{newDialogs(a, func(id string) application.WebviewWindowOptions {
		title := "Save download"
		if svc, err := a.service(); err == nil {
			if d, err := svc.Engine.Get(id); err == nil {
				title += ": " + d.Name()
			}
		}
		// Non-resizable so tiling window managers float it as a dialog; the page sizes it to fit
		// its content (fitWindow).
		return application.WebviewWindowOptions{
			Name:                "save-" + id,
			Title:               title,
			Width:               560,
			Height:              396,
			DisableResize:       true,
			MaximiseButtonState: application.ButtonHidden,
			URL:                 "/?save=" + url.QueryEscape(id),
		}
	})}
}

func (s *saveWindows) show(d engine.Download) {
	s.dialogs.show(d.ID)
}
