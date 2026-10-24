package service

import (
	"context"
	"errors"
	"net"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/Roham1299/snag/internal/config"
	"github.com/Roham1299/snag/internal/naming"
	"github.com/Roham1299/snag/internal/playlist"
	"github.com/Roham1299/snag/internal/subscriptions"
)

// subscriptionsDelay is the wait after start (loginDelay at login); newestToRead caps entries
// read from a newest-first list.
const (
	subscriptionsDelay = 20 * time.Second
	newestToRead       = 50
)

func (s *Service) startSubscriptions(ctx context.Context, atLogin bool) {
	s.checker = &subscriptions.Checker{
		Store: s.subs,
		Read:  s.readSubscription,
		Add:   s.addFromSubscription,
		Told:  s.subscriptionNews,
		Log:   s.log,
	}
	wait := subscriptionsDelay
	if atLogin {
		wait = loginDelay
	}
	go s.checker.Run(ctx, wait)
}

// readSubscription prefers the feed (cheap, and easy on the site's rate limits) and falls back to
// reading the list, since YouTube's feeds fail now and then.
func (s *Service) readSubscription(ctx context.Context, sub subscriptions.Subscription) (playlist.Playlist, error) {
	if feed := feedFor(sub.Source, sub.Feed); feed != "" {
		p, err := s.readFeed(ctx, sub.Source, feed)
		if err == nil {
			return p, nil
		}
		if sub.Podcast || sub.Feed == sub.Source {
			return playlist.Playlist{}, offline(err)
		}
		s.log.Info("a subscription's feed failed; reading its list instead", "title", sub.Title, "err", err)
	}
	req := playlist.Request{URL: sub.Source, Fresh: true}
	if sub.NewestFirst {
		req.Max = newestToRead
	}
	p, err := s.Playlists.List(ctx, req)
	return p, offline(err)
}

// feedFor returns "" when the feed holds more than the list: a YouTube channel's feed has all
// uploads, so its Shorts and Live tabs are read from the site (readFeed drops Shorts otherwise).
func feedFor(source, feed string) string {
	u, err := url.Parse(source)
	if err != nil || feed == "" || !strings.HasSuffix(u.Hostname(), "youtube.com") {
		return feed
	}
	switch path.Base(u.Path) {
	case "shorts", "streams", "live":
		return ""
	}
	return feed
}

func (s *Service) readFeed(ctx context.Context, source, feed string) (playlist.Playlist, error) {
	p, err := playlist.Feed{}.Read(ctx, feed)
	if err != nil {
		return p, err
	}
	p.NewestFirst = true
	if u, err := url.Parse(source); err == nil && strings.HasSuffix(u.Hostname(), "youtube.com") {
		p.Entries = slices.DeleteFunc(p.Entries, func(e playlist.Entry) bool {
			return strings.Contains(e.URL, "/shorts/")
		})
	}
	return p, nil
}

// seenInFeed lists the feed's entries when following a list, since the feed, read a moment later,
// may have newer entries than the list and those mustn't count as new.
func (s *Service) seenInFeed(source, feed string) []playlist.Entry {
	feed = feedFor(source, feed)
	if feed == "" || feed == source {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	p, err := s.readFeed(ctx, source, feed)
	if err != nil {
		return nil
	}
	return p.Entries
}

// offline maps no-network errors to subscriptions.ErrOffline.
func offline(err error) error {
	if err == nil {
		return nil
	}
	var dns *net.DNSError
	var op *net.OpError
	if errors.As(err, &dns) || (errors.As(err, &op) && op.Op == "dial") {
		return subscriptions.ErrOffline
	}
	text := strings.ToLower(err.Error())
	for _, sign := range []string{"name resolution", "failed to resolve", "network is unreachable", "no route to host"} {
		if strings.Contains(text, sign) {
			return subscriptions.ErrOffline
		}
	}
	return err
}

// addFromSubscription falls back to Main if the subscription's queue is gone.
func (s *Service) addFromSubscription(sub subscriptions.Subscription, p playlist.Playlist, indexes []int) (int, error) {
	queue := sub.Queue
	if _, ok := s.Config().Queue(queue); !ok {
		queue = config.MainQueue
	}
	reqs, err := playlist.Plan(p, playlist.Choice{
		Entries: indexes, Quality: sub.Quality, Directory: sub.Directory, Queue: queue, Naming: sub.Naming,
	})
	if err != nil {
		return 0, err
	}
	for i := range reqs {
		reqs[i].Subscription = sub.ID
	}
	added, _, err := s.Engine.AddMany(reqs)
	if err != nil {
		return 0, err
	}
	if queue != config.MainQueue {
		s.Engine.StartQueue(queue)
	}
	return added, nil
}

func (s *Service) subscriptionNews(sub subscriptions.Subscription, titles []string) {
	s.mu.Lock()
	listeners := append([]func(subscriptions.Subscription, []string){}, s.onSubscriptionNews...)
	s.mu.Unlock()
	for _, f := range listeners {
		f(sub, titles)
	}
}

// OnSubscriptionNews calls f whenever a subscription adds downloads, with their titles.
func (s *Service) OnSubscriptionNews(f func(sub subscriptions.Subscription, titles []string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onSubscriptionNews = append(s.onSubscriptionNews, f)
}

// OnSubscriptionsChanged calls f whenever a subscription is added, changed or removed.
func (s *Service) OnSubscriptionsChanged(f func()) {
	s.subs.OnChange(f)
}

// Subscriptions lists the lists Snag follows.
func (s *Service) Subscriptions() []subscriptions.Subscription {
	return s.subs.List()
}

// Following returns the subscription to the list at source, if Snag follows it.
func (s *Service) Following(source string) (subscriptions.Subscription, bool) {
	return s.subs.Following(source)
}

// SubscriptionChange holds the editable fields of a subscription.
type SubscriptionChange struct {
	Title      string                `json:"title"`
	Quality    string                `json:"quality"`
	Directory  string                `json:"directory"`
	Queue      string                `json:"queue"`
	Naming     naming.Template       `json:"naming"`
	EveryHours int                   `json:"everyHours"`
	Paused     bool                  `json:"paused"`
	Filters    subscriptions.Filters `json:"filters"`
}

// ChangeSubscription changes a subscription and returns it as changed.
func (s *Service) ChangeSubscription(id string, c SubscriptionChange) (subscriptions.Subscription, error) {
	if _, ok := playlist.QualityOf(c.Quality); !ok {
		return subscriptions.Subscription{}, errors.New("unknown quality")
	}
	if err := c.Naming.Check(); err != nil {
		return subscriptions.Subscription{}, err
	}
	dir := expandHome(strings.TrimSpace(c.Directory))
	if dir == "" {
		return subscriptions.Subscription{}, errors.New("choose a folder")
	}
	now := time.Now()
	return s.subs.Update(id, func(sub *subscriptions.Subscription) {
		resumed := sub.Paused && !c.Paused
		sooner := c.EveryHours > 0 && c.EveryHours < sub.EveryHours
		sub.Title = strings.TrimSpace(c.Title)
		if sub.Title == "" {
			sub.Title = "Playlist"
		}
		sub.Quality, sub.Directory, sub.Queue, sub.Naming = c.Quality, dir, c.Queue, c.Naming
		sub.EveryHours, sub.Paused, sub.Filters = c.EveryHours, c.Paused, c.Filters
		if resumed || sooner {
			sub.Next = now.Add(time.Duration(sub.EveryHours) * time.Hour / 2)
		}
	})
}

// CheckSubscription looks at a subscription now.
func (s *Service) CheckSubscription(id string) error {
	return s.checker.CheckNow(id)
}

// CheckingSubscription reports whether a subscription is being looked at now.
func (s *Service) CheckingSubscription(id string) bool {
	return s.checker.Busy(id)
}

// Unsubscribe stops following a list; its downloads stay.
func (s *Service) Unsubscribe(id string) error {
	return s.subs.Remove(id)
}
