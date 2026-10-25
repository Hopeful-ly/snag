package app

import (
	"errors"
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"github.com/Roham1299/snag/internal/feed"
	"github.com/Roham1299/snag/internal/update"
)

// Update is a newer release of Snag, as About shows it.
type Update struct {
	Version string `json:"version"`
	// Ready is whether its installer is downloaded and checked, so a restart installs it.
	Ready bool `json:"ready"`
}

// SetUpdate records the newest release (nil when this one is the latest) and the path of its
// downloaded installer ("" if there's none). With tell, it also notifies the user if
// notifications are on.
func (a *App) SetUpdate(r *feed.Release, installer string, tell bool) {
	a.mu.Lock()
	a.update, a.installer = r, installer
	svc := a.svc
	a.mu.Unlock()
	a.tray.changed()
	if r == nil || !tell || svc == nil || !svc.Config().Notify {
		return
	}
	body := "Get it from Settings, About."
	if installer != "" {
		body = "Restart Snag to update, from the tray or Settings, About."
	}
	a.notify("snag-update", "Snag "+r.Version+" is out", body)
}

func (a *App) notify(id, title, body string) {
	err := a.notes.SendNotification(notifications.NotificationOptions{ID: id, Title: title, Body: body})
	if err != nil {
		slog.Warn("couldn't show a notification", "err", err)
	}
}

func (a *App) newerRelease() (*feed.Release, string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.update, a.installer
}

// installUpdate installs the downloaded update and quits, for the new Snag to start.
func (a *App) installUpdate() error {
	r, installer := a.newerRelease()
	f, ok := update.For(r)
	if installer == "" || !ok {
		return errors.New("there's no update to install")
	}
	if err := update.Install(installer, f.SHA256); err != nil {
		return err
	}
	a.Quit()
	return nil
}

// Update returns the newer release, or nil.
func (s *SystemService) Update() *Update {
	if r, installer := s.app.newerRelease(); r != nil {
		return &Update{Version: r.Version, Ready: installer != ""}
	}
	return nil
}

// InstallUpdate installs the downloaded update and restarts Snag into it.
func (s *SystemService) InstallUpdate() error {
	return s.app.installUpdate()
}

// OpenUpdatePage opens the newer release's page (from the signed feed) in the browser.
func (s *SystemService) OpenUpdatePage() error {
	r, _ := s.app.newerRelease()
	if r == nil {
		return nil
	}
	return s.app.wails.Browser.OpenURL(r.Page)
}
