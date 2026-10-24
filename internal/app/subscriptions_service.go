package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"github.com/Roham1299/snag/internal/playlist"
	"github.com/Roham1299/snag/internal/service"
	"github.com/Roham1299/snag/internal/subscriptions"
)

// eventSubscriptions carries the subscription list when it changes.
const eventSubscriptions = "subscriptions:changed"

// Subscription is a list Snag follows, as the window shows it.
type Subscription struct {
	ID          string                `json:"id"`
	Title       string                `json:"title"`
	Source      string                `json:"source"`
	Site        string                `json:"site"`
	Podcast     bool                  `json:"podcast"`
	Quality     string                `json:"quality"`
	Qualities   []Quality             `json:"qualities"`
	Directory   string                `json:"directory"`
	Queue       string                `json:"queue"`
	Naming      string                `json:"naming"`
	EveryHours  int                   `json:"everyHours"`
	Paused      bool                  `json:"paused"`
	Filters     subscriptions.Filters `json:"filters"`
	Checked     int64                 `json:"checked"` // ms since 1970; 0 before the first check
	Next        int64                 `json:"next"`
	Error       string                `json:"error"`
	Added       int                   `json:"added"`
	Last        int                   `json:"last"`
	Checking    bool                  `json:"checking"`
	Seen        int                   `json:"seen"` // entries known
	CreatedTime int64                 `json:"created"`
}

// SubscriptionChange holds the editable fields of a subscription.
type SubscriptionChange = service.SubscriptionChange

func subscriptionView(s subscriptions.Subscription, checking bool) Subscription {
	v := Subscription{
		ID: s.ID, Title: s.Title, Source: s.Source, Site: s.Site, Podcast: s.Podcast,
		Quality: s.Quality, Qualities: playlist.QualitiesFor(playlist.Playlist{Podcast: s.Podcast}),
		Directory: s.Directory, Queue: s.Queue, Naming: string(s.Naming), EveryHours: s.EveryHours,
		Paused: s.Paused, Filters: s.Filters, Error: s.Error, Added: s.Added, Last: s.Last,
		Checking: checking, Seen: len(s.Seen), CreatedTime: s.Created.UnixMilli(),
	}
	if !s.Checked.IsZero() {
		v.Checked = s.Checked.UnixMilli()
	}
	if !s.Next.IsZero() {
		v.Next = s.Next.UnixMilli()
	}
	return v
}

// SubscriptionsService backs the subscriptions UI.
type SubscriptionsService struct {
	app *App
}

// List returns the subscriptions in the order they were added.
func (s *SubscriptionsService) List() ([]Subscription, error) {
	svc, err := s.app.service()
	if err != nil {
		return nil, err
	}
	return subscriptionViews(svc), nil
}

func subscriptionViews(svc *service.Service) []Subscription {
	list := svc.Subscriptions()
	out := make([]Subscription, len(list))
	for i, sub := range list {
		out[i] = subscriptionView(sub, svc.CheckingSubscription(sub.ID))
	}
	return out
}

// Change changes a subscription and returns it as changed.
func (s *SubscriptionsService) Change(id string, change SubscriptionChange) (Subscription, error) {
	svc, err := s.app.service()
	if err != nil {
		return Subscription{}, err
	}
	sub, err := svc.ChangeSubscription(id, change)
	return subscriptionView(sub, svc.CheckingSubscription(id)), err
}

// CheckNow checks a subscription for new entries now.
func (s *SubscriptionsService) CheckNow(id string) error {
	svc, err := s.app.service()
	if err != nil {
		return err
	}
	return svc.CheckSubscription(id)
}

// Find opens the playlist window for the channel, playlist or podcast at link, preset to
// subscribe with nothing ticked.
func (s *SubscriptionsService) Find(link string) error {
	svc, err := s.app.service()
	if err != nil {
		return err
	}
	if u, err := url.Parse(strings.TrimSpace(link)); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("that doesn't look like a link (it should start with http:// or https://)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), addTimeout)
	defer cancel()
	p, err := svc.ListPlaylist(ctx, playlist.Request{URL: strings.TrimSpace(link), Follow: true})
	if errors.Is(err, playlist.ErrNotAList) {
		return errors.New("Snag found no channel, playlist or podcast there") //nolint:staticcheck // a name
	}
	if err != nil {
		return err
	}
	s.app.ShowPlaylist(p.ID)
	return nil
}

// OpenPage opens the page a subscription follows in the browser.
func (s *SubscriptionsService) OpenPage(id string) error {
	svc, err := s.app.service()
	if err != nil {
		return err
	}
	for _, sub := range svc.Subscriptions() {
		if sub.ID == id {
			return s.app.wails.Browser.OpenURL(sub.Source)
		}
	}
	return subscriptions.ErrNotFound
}

// Remove stops following a list; what it downloaded stays.
func (s *SubscriptionsService) Remove(id string) error {
	svc, err := s.app.service()
	if err != nil {
		return err
	}
	return svc.Unsubscribe(id)
}

func (a *App) followSubscriptions(svc *service.Service) {
	svc.OnSubscriptionsChanged(func() { a.wails.Event.Emit(eventSubscriptions, subscriptionViews(svc)) })
	svc.OnSubscriptionNews(func(sub subscriptions.Subscription, titles []string) {
		if !svc.Config().Notify {
			return
		}
		err := a.notes.SendNotification(notifications.NotificationOptions{
			ID:    "snag-subscription-" + sub.ID,
			Title: fmt.Sprintf("%s: %s", sub.Title, newThings(sub, len(titles))),
			Body:  strings.Join(titles[:min(len(titles), 3)], "\n") + more(len(titles)),
		})
		if err != nil {
			slog.Warn("couldn't show a notification", "err", err)
		}
	})
}

// newThings formats "2 new videos", "1 new episode".
func newThings(sub subscriptions.Subscription, n int) string {
	thing := "video"
	switch {
	case sub.Podcast:
		thing = "episode"
	case sub.Quality == "mp3" || sub.Quality == "m4a":
		thing = "song"
	}
	if n != 1 {
		thing += "s"
	}
	return fmt.Sprintf("%d new %s", n, thing)
}

func more(n int) string {
	if n <= 3 {
		return ""
	}
	return fmt.Sprintf("\nand %d more", n-3)
}
