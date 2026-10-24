// Package app is Snag's desktop shell: window, tray, notifications and the services the web UI
// calls. Download logic belongs in package service.
package app

import (
	"errors"
	"io/fs"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/feed"
	"github.com/Roham1299/snag/internal/platform"
	"github.com/Roham1299/snag/internal/service"
	"github.com/Roham1299/snag/packaging/icons"
)

const (
	name     = "Snag"
	uniqueID = "app.snag.desktop"
)

const backgroundFlag = "--background"

var errNotReady = errors.New("snag is still starting")

// App is the running desktop application.
type App struct {
	wails     *application.App
	window    *application.WebviewWindow
	tray      *tray
	saves     *saveWindows
	playlists *dialogs
	links     *dialogs
	notes     *notifications.NotificationService

	mu  sync.Mutex
	svc *service.Service
	// update is a newer release; nil when this one is the latest.
	update *feed.Release
}

// Options configure New.
type Options struct {
	// Background starts the app hidden in the tray.
	Background bool
}

// New builds the application around the web UI assets. If Snag is already running, it hands
// over to that instance and exits.
func New(assets fs.FS, opts Options) *App {
	a := &App{notes: notifications.New()}
	a.saves = newSaveWindows(a)
	a.playlists = newPlaylistWindows(a)
	a.links = newLinksWindows(a)
	platform.InstallAppIcon(icons.App, icons.SVG)
	a.wails = application.New(application.Options{
		Name:        name,
		Description: "Download manager",
		Icon:        icons.App,
		Linux: application.LinuxOptions{
			ApplicationID: uniqueID,
			ProgramName:   uniqueID,
		},
		Services: []application.Service{
			application.NewService(&SystemService{app: a}),
			application.NewService(&DownloadsService{app: a}),
			application.NewService(&SettingsService{app: a}),
			application.NewService(&SetupService{app: a}),
			application.NewService(&QueuesService{app: a}),
			application.NewService(&PlaylistsService{app: a}),
			application.NewService(&LinksService{app: a}),
			application.NewService(&SubscriptionsService{app: a}),
			application.NewService(a.notes),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: uniqueID,
			OnSecondInstanceLaunch: func(second application.SecondInstanceData) {
				if !slices.Contains(second.Args, backgroundFlag) && !slices.Contains(second.Args, platform.AtLoginFlag) {
					a.show()
				}
			},
		},
		// Snag keeps running in the tray when its window is closed.
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})
	a.window = newMainWindow(a.wails, !opts.Background)
	a.tray = newTray(a)
	a.setUpNotifications()
	return a
}

// Attach connects the app to the service so the window, tray and notifications follow it.
func (a *App) Attach(svc *service.Service) {
	a.mu.Lock()
	a.svc = svc
	a.mu.Unlock()
	svc.SetWindow(a)
	a.followSubscriptions(svc)
	svc.OnSettingsChanged(a.settingsChanged)
	svc.OnBrowserContact(func() { a.wails.Event.Emit(eventConnected, true) })
	svc.OnQueuesChanged(func() { a.wails.Event.Emit(eventQueues, svc.Queues()) })
	svc.OnCountdown(func(c *service.Countdown) {
		a.wails.Event.Emit(eventCountdown, c)
		if c != nil {
			a.show() // so the countdown can be cancelled
		}
	})
	svc.OnQuit(a.Quit)
	go a.follow(svc)
}

func (a *App) service() (*service.Service, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.svc == nil {
		return nil, errNotReady
	}
	return a.svc, nil
}

// Run starts the application and blocks until it quits.
func (a *App) Run() error {
	return a.wails.Run()
}

// Show brings the main window up, or a pending download's Save window.
func (a *App) Show(downloadID string) {
	if svc, err := a.service(); err == nil && downloadID != "" {
		if d, err := svc.Engine.Get(downloadID); err == nil && d.Status == engine.StatusPending {
			a.saves.show(d)
			return
		}
	}
	a.show()
}

// mapDelay is how long a hidden window takes to be mapped and focusable again.
const mapDelay = 150 * time.Millisecond

// show focuses the window even on another workspace, which the desktop switches to.
func (a *App) show() {
	a.window.Show()
	a.window.Restore()
	a.window.Focus()
	platform.BringForward(os.Getpid(), name)
	time.AfterFunc(mapDelay, func() { platform.BringForward(os.Getpid(), name) })
}

// Quit closes the window and stops the application.
func (a *App) Quit() {
	a.wails.Quit()
}
