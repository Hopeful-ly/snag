package service

import (
	"cmp"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Roham1299/snag/internal/config"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/naming"
	"github.com/Roham1299/snag/internal/playlist"
	"github.com/Roham1299/snag/internal/subscriptions"
)

// playlistAtOnce is the concurrency of a queue made for a list.
const playlistAtOnce = 2

// ErrNoPlaylist means the list is no longer kept.
var ErrNoPlaylist = errors.New("that playlist is gone; open it again from its page")

// PlaylistAdded reports the result of downloading a list.
type PlaylistAdded struct {
	Queue config.Queue `json:"queue"`
	Added int          `json:"added"`
	// Already counts entries that were already unfinished in the list.
	Already   int  `json:"already"`
	Following bool `json:"following,omitempty"`
	// Note says what didn't go as asked, such as falling back to Main.
	Note string `json:"note,omitempty"`
}

// ListPlaylist finds and keeps the list a page is or belongs to. It returns playlist.ErrNotAList
// for a page of one video.
func (s *Service) ListPlaylist(ctx context.Context, req playlist.Request) (playlist.Playlist, error) {
	p, err := s.Playlists.List(ctx, req)
	if err != nil {
		return playlist.Playlist{}, err
	}
	p.Follow = req.Follow
	return s.playlists.Put(p), nil
}

// Playlist returns a list kept by ListPlaylist.
func (s *Service) Playlist(id string) (playlist.Playlist, error) {
	p, ok := s.playlists.Get(id)
	if !ok {
		return playlist.Playlist{}, ErrNoPlaylist
	}
	return p, nil
}

// PlaylistSubfolder is the list's own folder name.
func PlaylistSubfolder(p playlist.Playlist) string {
	return naming.Safe(p.Title, "Playlist")
}

// PlaylistFolder is the default folder for a list's downloads: its own subfolder of the download
// folder, or the download folder itself for a single song.
func (s *Service) PlaylistFolder(p playlist.Playlist) string {
	dir := s.Config().DownloadDir
	if len(p.Entries) == 1 {
		return dir
	}
	return filepath.Join(dir, PlaylistSubfolder(p))
}

// PlaylistNaming is the default naming for a list's downloads: the queue's, else the settings'
// for that kind of list and quality.
func (s *Service) PlaylistNaming(p playlist.Playlist, quality, queue string) naming.Template {
	cfg := s.Config()
	if q, ok := cfg.Queue(queue); ok && q.Naming != "" {
		return q.Naming
	}
	switch playlist.KindOf(p, quality) {
	case playlist.KindEpisodes:
		return cfg.Naming.Episodes
	case playlist.KindSongs:
		return cfg.Naming.Songs
	}
	return cfg.Naming.Videos
}

// PlaylistHave reports, per entry, whether it is already downloaded at that quality.
func (s *Service) PlaylistHave(id, quality string) ([]bool, error) {
	p, err := s.Playlist(id)
	if err != nil {
		return nil, err
	}
	q, ok := playlist.QualityOf(quality)
	if !ok {
		return nil, errors.New("unknown quality")
	}
	have := make([]bool, len(p.Entries))
	for i, e := range p.Entries {
		_, have[i] = s.Engine.Have(engine.HaveQuery{
			URL: e.URL, Page: e.URL, Title: e.Title, Label: q.Label, Format: q.Format, Audio: q.Audio,
		})
	}
	return have, nil
}

// DownloadPlaylist downloads the chosen entries in order, in the chosen queue or one named after
// the list, and starts that queue. The queue remembers the naming for the next list.
func (s *Service) DownloadPlaylist(id string, c playlist.Choice) (PlaylistAdded, error) {
	return s.downloadPlaylist(id, c, 0)
}

// FollowPlaylist downloads like DownloadPlaylist (none chosen is fine) and then downloads new
// entries every everyHours.
func (s *Service) FollowPlaylist(id string, c playlist.Choice, everyHours int) (PlaylistAdded, error) {
	return s.downloadPlaylist(id, c, max(everyHours, 1))
}

func (s *Service) downloadPlaylist(id string, c playlist.Choice, followEvery int) (PlaylistAdded, error) {
	p, err := s.Playlist(id)
	if err != nil {
		return PlaylistAdded{}, err
	}
	if followEvery > 0 && p.Via == playlist.ViaBrowser {
		return PlaylistAdded{}, errors.New("Snag can't follow a streaming site's show yet: its episodes only come by playing them") //nolint:staticcheck // a name
	}
	c.Directory = cmp.Or(expandHome(strings.TrimSpace(c.Directory)), s.PlaylistFolder(p))
	c.Naming = cmp.Or(naming.Template(strings.TrimSpace(string(c.Naming))), s.PlaylistNaming(p, c.Quality, c.Queue))
	var reqs []engine.AddRequest
	if len(c.Entries) > 0 || followEvery == 0 {
		if reqs, err = playlist.Plan(p, c); err != nil {
			return PlaylistAdded{}, err
		}
	} else if err := c.Naming.Check(); err != nil {
		return PlaylistAdded{}, err
	}
	var added PlaylistAdded
	added.Queue, added.Note = s.playlistQueue(c.Queue, p)
	if added.Queue.ID != config.MainQueue && added.Queue.Naming != c.Naming {
		added.Queue.Naming = c.Naming
		if q, err := s.SaveQueue(added.Queue); err == nil {
			added.Queue = q
		}
	}
	// Subscribe first, so the entries chosen now count as already downloaded by it.
	subscription := ""
	if followEvery > 0 {
		c.Queue = added.Queue.ID
		seen := p
		seen.Entries = append(slices.Clone(p.Entries), s.seenInFeed(p.Page, p.Feed)...)
		sub, err := s.subs.Add(subscriptions.New(seen, c, followEvery, time.Now()))
		if err != nil {
			return added, err
		}
		subscription, added.Following = sub.ID, true
	}
	for i := range reqs {
		reqs[i].Queue, reqs[i].Subscription = added.Queue.ID, subscription
	}
	added.Added, added.Already, err = s.Engine.AddMany(reqs)
	if err != nil {
		return added, err
	}
	if added.Queue.ID != config.MainQueue {
		s.Engine.StartQueue(added.Queue.ID)
	}
	recent := c.Directory
	if filepath.Base(recent) == PlaylistSubfolder(p) {
		recent = filepath.Dir(recent)
	}
	if _, err := s.Update(func(cfg *config.Config) { cfg.AddRecentDir(recent) }); err != nil {
		s.log.Warn("couldn't remember the folder", "dir", recent, "err", err)
	}
	return added, nil
}

// playlistQueue returns the queue with id, else the one named after the list (creating it),
// else Main with a note saying why.
func (s *Service) playlistQueue(id string, p playlist.Playlist) (config.Queue, string) {
	cfg := s.Config()
	if q, ok := cfg.Queue(id); ok {
		return q, ""
	}
	name := cmp.Or(p.Title, "Playlist")
	for _, q := range cfg.Queues {
		if strings.EqualFold(q.Name, name) {
			return q, ""
		}
	}
	q, err := s.SaveQueue(config.Queue{
		Name: name, AtOnce: playlistAtOnce, Retries: config.DefaultRetries, RetryWait: config.DefaultRetryWait,
	})
	if err != nil {
		main, _ := cfg.Queue(config.MainQueue)
		return main, "They are in Main: " + err.Error()
	}
	return q, ""
}

// expandHome expands a leading "~/".
func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`) {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}
