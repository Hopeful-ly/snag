package app

import (
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"github.com/Roham1299/snag/internal/feed"
)

// Update is a newer release of Snag, as About shows it.
type Update struct {
	Version string `json:"version"`
}

// SetUpdate records the newest release (nil when this one is the latest). With tell, it also
// notifies the user if notifications are on.
func (a *App) SetUpdate(r *feed.Release, tell bool) {
	a.mu.Lock()
	a.update = r
	svc := a.svc
	a.mu.Unlock()
	if r == nil || !tell || svc == nil || !svc.Config().Notify {
		return
	}
	err := a.notes.SendNotification(notifications.NotificationOptions{
		ID:    "snag-update",
		Title: "Snag " + r.Version + " is out",
		Body:  "Get it from Settings, About.",
	})
	if err != nil {
		slog.Warn("couldn't show a notification", "err", err)
	}
}

func (a *App) newerRelease() *feed.Release {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.update
}

// Update returns the newer release, or nil.
func (s *SystemService) Update() *Update {
	if r := s.app.newerRelease(); r != nil {
		return &Update{Version: r.Version}
	}
	return nil
}

// OpenUpdatePage opens the newer release's page (from the signed feed) in the browser.
func (s *SystemService) OpenUpdatePage() error {
	r := s.app.newerRelease()
	if r == nil {
		return nil
	}
	return s.app.wails.Browser.OpenURL(r.Page)
}
