package app

import (
	"os"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/Roham1299/snag/internal/platform"
	"github.com/Roham1299/snag/packaging/icons"
)

// dialogs are windows of one kind, one per key (a download, a playlist). Closing one only
// defers the question; asking again brings it back.
type dialogs struct {
	app     *App
	options func(key string) application.WebviewWindowOptions

	mu   sync.Mutex
	open map[string]*dialog
}

type dialog struct {
	window *application.WebviewWindow
	title  string
}

func newDialogs(a *App, options func(key string) application.WebviewWindowOptions) *dialogs {
	return &dialogs{app: a, options: options, open: map[string]*dialog{}}
}

func (s *dialogs) show(key string) {
	s.mu.Lock()
	w := s.open[key]
	if w == nil {
		w = s.create(key)
		s.open[key] = w
	}
	s.mu.Unlock()

	application.InvokeSync(func() {
		w.window.Show()
		w.window.Focus()
	})
	platform.BringForward(os.Getpid(), w.title)
	time.AfterFunc(mapDelay, func() { platform.BringForward(os.Getpid(), w.title) })
}

// create makes the window for key. The lock must be held.
func (s *dialogs) create(key string) *dialog {
	opts := s.options(key)
	opts.BackgroundColour = application.NewRGB(27, 30, 36)
	opts.InitialPosition = application.WindowCentered
	opts.Linux = application.LinuxWindow{Icon: icons.App}
	w := &dialog{title: opts.Title}
	application.InvokeSync(func() {
		w.window = s.app.wails.Window.NewWithOptions(opts)
		w.window.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
			s.mu.Lock()
			if s.open[key] == w {
				delete(s.open, key)
			}
			s.mu.Unlock()
		})
	})
	return w
}

func (s *dialogs) close(key string) {
	s.mu.Lock()
	w := s.open[key]
	delete(s.open, key)
	s.mu.Unlock()
	if w != nil {
		application.InvokeSync(w.window.Close)
	}
}
