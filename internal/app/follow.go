package app

import (
	"log/slog"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"github.com/Roham1299/snag/internal/config"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/service"
)

// Notification categories and actions for finished downloads.
const (
	doneCategory = "snag-download-done"
	// checkCategory adds a Check button when a checksum was found.
	checkCategory = "snag-download-done-check"
	actionOpen    = "open"
	actionReveal  = "reveal"
	actionCheck   = "check"
)

// follow relays engine changes to the windows, tray and notifications, and opens or closes Save
// windows for pending downloads.
func (a *App) follow(svc *service.Service) {
	changes, stop := svc.Engine.Subscribe()
	defer stop()

	finished := map[string]bool{}
	for _, d := range svc.Engine.List() {
		finished[d.ID] = d.Status == engine.StatusDone
		if d.Status == engine.StatusPending {
			a.saves.show(d)
		}
	}
	a.tray.changed()

	for change := range changes {
		if change.Kind == engine.EventReset {
			if svc, err := a.service(); err == nil {
				a.wails.Event.Emit(eventReset, viewsOf(svc.Engine.List()))
			}
			a.tray.changed()
			continue
		}
		d := change.Download
		if change.Kind == engine.EventRemoved {
			delete(finished, d.ID)
			a.saves.close(d.ID)
			a.wails.Event.Emit(eventRemoved, d.ID)
		} else {
			a.wails.Event.Emit(eventChanged, viewOf(d))
			switch {
			case change.Kind == engine.EventAdded && d.Status == engine.StatusPending:
				a.saves.show(d)
			case d.Status != engine.StatusPending:
				a.saves.close(d.ID)
			}
			// Hold the notification while looking for a checksum, so it can offer a check.
			if d.Status == engine.StatusDone && !finished[d.ID] && d.Check.Result != engine.CheckLooking {
				finished[d.ID] = true
				if svc.Config().Notify {
					a.notifyFinished(d)
				}
			}
		}
		a.tray.changed()
	}
}

func (a *App) settingsChanged(c config.Config) {
	a.wails.Event.Emit(eventSettings, c)
	if svc, err := a.service(); err == nil {
		a.wails.Event.Emit(eventQueues, svc.Queues())
	}
	a.tray.changed()
}

// setUpNotifications registers the notification actions once the app has started, which the
// notification service requires.
func (a *App) setUpNotifications() {
	a.wails.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		for _, category := range []notifications.NotificationCategory{
			{ID: doneCategory, Actions: []notifications.NotificationAction{
				{ID: actionOpen, Title: "Open"},
				{ID: actionReveal, Title: "Show in folder"},
			}},
			{ID: checkCategory, Actions: []notifications.NotificationAction{
				{ID: actionOpen, Title: "Open"},
				{ID: actionReveal, Title: "Show in folder"},
				{ID: actionCheck, Title: "Check"},
			}},
		} {
			if err := a.notes.RegisterNotificationCategory(category); err != nil {
				slog.Warn("couldn't set up notifications", "err", err)
			}
		}
		if _, err := a.notes.RequestNotificationAuthorization(); err != nil {
			slog.Debug("notifications not authorized", "err", err)
		}
	})
	a.notes.OnNotificationResponse(func(result notifications.NotificationResult) {
		if result.Error != nil {
			return
		}
		svc, err := a.service()
		if err != nil {
			return
		}
		id, _ := result.Response.UserInfo["id"].(string)
		switch result.Response.ActionIdentifier {
		case actionOpen:
			err = svc.Open(id)
		case actionReveal:
			err = svc.Reveal(id)
		case actionCheck:
			a.show() // the check's result shows there
			err = svc.Engine.Check(id, "")
		default:
			a.show()
		}
		if err != nil {
			slog.Warn("couldn't act on a notification", "err", err)
		}
	})
}

func (a *App) notifyFinished(d engine.Download) {
	body, category := d.Name(), doneCategory
	if c := d.Check; c.Found() {
		body += "\n" + checksumFound(c)
		category = checkCategory
	}
	err := a.notes.SendNotificationWithActions(notifications.NotificationOptions{
		ID:         "snag-" + d.ID,
		Title:      "Download complete",
		Body:       body,
		CategoryID: category,
		Data:       map[string]any{"id": d.ID},
	})
	if err != nil {
		slog.Warn("couldn't show a notification", "err", err)
	}
}

// checksumFound says what checksum was found for a file, and where: "SHA-256 found on the page".
func checksumFound(c engine.Check) string {
	where := map[string]string{
		engine.FromPage:   "on the page",
		engine.FromServer: "from its server",
		engine.FromGitHub: "on GitHub",
		engine.FromFile:   "in a checksum file",
	}[c.From]
	return strings.TrimSpace(c.Algo.Name() + " found " + where)
}
