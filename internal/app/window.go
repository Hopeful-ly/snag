package app

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/Roham1299/snag/packaging/icons"
)

func newMainWindow(wails *application.App, visible bool) *application.WebviewWindow {
	window := wails.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            name,
		Width:            1000,
		Height:           660,
		MinWidth:         780,
		MinHeight:        500,
		InitialPosition:  application.WindowCentered,
		BackgroundColour: application.NewRGB(18, 20, 24),
		URL:              "/",
		Hidden:           !visible,
		Linux:            application.LinuxWindow{Icon: icons.App},
	})

	// Closing only hides the window; the tray brings it back.
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		window.Hide()
		e.Cancel()
	})
	return window
}
