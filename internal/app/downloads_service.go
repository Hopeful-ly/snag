package app

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/Roham1299/snag/internal/applemusic"
	"github.com/Roham1299/snag/internal/checksum"
	"github.com/Roham1299/snag/internal/config"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/platform"
	"github.com/Roham1299/snag/internal/playlist"
	"github.com/Roham1299/snag/internal/service"
	"github.com/Roham1299/snag/internal/spotify"
)

const (
	addTimeout    = 2 * time.Minute
	searchTimeout = time.Minute
	hashTimeout   = 10 * time.Minute // hashing a very large file
)

// compatibleSort makes yt-dlp pick the most compatible format for a pasted video page.
const compatibleSort = "vcodec:h264,acodec:aac,ext:mp4:m4a"

// DownloadsService backs the main window's download list.
type DownloadsService struct {
	app *App
}

// Added is the result of adding a link.
type Added struct {
	Download  Download `json:"download"`
	Duplicate bool     `json:"duplicate"` // already in the list, unfinished
	// Playlist is set when the link is a list: nothing was added and the list's window opened.
	Playlist string `json:"playlist,omitempty"`
}

// SaveAnswer is the Save dialog's answer for a pending download.
type SaveAnswer struct {
	Directory string `json:"directory"`
	Filename  string `json:"filename"` // "" keeps the name Snag picks
	Start     bool   `json:"start"`    // false adds it paused
	// Remember makes the folder the default and stops asking for new downloads.
	Remember bool `json:"remember"`
	Replace  bool `json:"replace"`
	// Queue "" means Main.
	Queue string `json:"queue"`
}

// Clash is a name already taken in the folder the Save dialog points at.
type Clash = engine.Clash

// List returns every download, newest first.
func (s *DownloadsService) List() ([]Download, error) {
	svc, err := s.app.service()
	if err != nil {
		return nil, err
	}
	return viewsOf(svc.Engine.List()), nil
}

// Add adds a link: a file, a video page, a stream or a list.
func (s *DownloadsService) Add(link string) (Added, error) {
	return s.app.addLink(link, false)
}

// addLink opens a list's window instead of adding it. confirmed skips the Save dialog, as for a
// link from a phone.
func (a *App) addLink(link string, confirmed bool) (Added, error) {
	svc, err := a.service()
	if err != nil {
		return Added{}, err
	}
	if u, err := url.Parse(link); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Added{}, errors.New("that doesn't look like a link (it should start with http:// or https://)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), addTimeout)
	defer cancel()
	// Spotify and Apple Music links always go through the playlist window, even a single song,
	// since the audio is matched on YouTube Music.
	_, fromSpotify := spotify.Parse(link)
	_, fromApple := applemusic.Parse(link)
	if fromSpotify || fromApple {
		return a.openList(ctx, svc, link)
	}
	if playlist.LooksLikeFeed(link) {
		if added, err := a.openList(ctx, svc, link); err == nil {
			return added, nil
		}
	}
	req := engine.AddRequest{URL: link, Kind: engine.KindHTTP, Confirmed: confirmed}
	if svc.Prober.IsStream(ctx, link) {
		// Cheap: the prober has just cached the page.
		if p, err := svc.ListPlaylist(ctx, playlist.Request{URL: link}); err == nil {
			a.ShowPlaylist(p.ID)
			return Added{Playlist: p.ID}, nil
		}
		req.Kind, req.Sort = engine.KindStream, compatibleSort
	}
	d, duplicate, err := svc.Engine.Add(req)
	if err != nil {
		return Added{}, err
	}
	return Added{Download: viewOf(d), Duplicate: duplicate}, nil
}

func (a *App) openList(ctx context.Context, svc *service.Service, link string) (Added, error) {
	p, err := svc.ListPlaylist(ctx, playlist.Request{URL: link})
	if err != nil {
		return Added{}, err
	}
	a.ShowPlaylist(p.ID)
	return Added{Playlist: p.ID}, nil
}

// Get returns one download.
func (s *DownloadsService) Get(id string) (Download, error) {
	svc, err := s.app.service()
	if err != nil {
		return Download{}, err
	}
	d, err := svc.Engine.Get(id)
	return viewOf(d), err
}

// Found is one result of looking for a song somewhere other than YouTube Music.
type Found = service.Found

// SearchElsewhere searches place ("youtube" or "soundcloud") for a song YouTube Music didn't have.
func (s *DownloadsService) SearchElsewhere(id, place string) ([]Found, error) {
	svc, err := s.app.service()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), searchTimeout)
	defer cancel()
	return svc.SearchElsewhere(ctx, id, place)
}

// UseFound points an unmatched song's download at address and queues it. remember makes place
// the queue's fallback.
func (s *DownloadsService) UseFound(id, address, place string, remember bool) error {
	svc, err := s.app.service()
	if err != nil {
		return err
	}
	return svc.UseFound(id, address, place, remember)
}

// Choose opens or focuses a pending download's Save window.
func (s *DownloadsService) Choose(id string) error {
	svc, err := s.app.service()
	if err != nil {
		return err
	}
	d, err := svc.Engine.Get(id)
	if err != nil {
		return err
	}
	if d.Status != engine.StatusPending {
		return errors.New("it already has a place to go")
	}
	s.app.saves.show(d)
	return nil
}

// Pause pauses a download.
func (s *DownloadsService) Pause(id string) error {
	return s.withEngine(func(e *engine.Engine) error { return e.Pause(id) })
}

// Resume resumes a paused download, or tries a failed one again.
func (s *DownloadsService) Resume(id string) error {
	return s.withEngine(func(e *engine.Engine) error { return e.Resume(id) })
}

// Remove takes a download off the list and deletes its partial data; deleteFile also deletes a
// finished file.
func (s *DownloadsService) Remove(id string, deleteFile bool) error {
	return s.withEngine(func(e *engine.Engine) error { return e.Remove(id, deleteFile) })
}

// Confirm answers the Save dialog for a pending download.
func (s *DownloadsService) Confirm(id string, answer SaveAnswer) error {
	svc, err := s.app.service()
	if err != nil {
		return err
	}
	err = svc.Engine.Confirm(id, engine.ConfirmRequest{
		Directory: answer.Directory, Filename: answer.Filename, Paused: !answer.Start,
		Replace: answer.Replace, Queue: answer.Queue,
	})
	if err != nil {
		return err
	}
	d, err := svc.Engine.Get(id)
	if err != nil {
		return err
	}
	_, err = svc.Update(func(c *config.Config) {
		c.AddRecentDir(d.Directory)
		if answer.Remember {
			c.DownloadDir, c.Ask = d.Directory, false
		}
	})
	return err
}

// Clash reports an existing file at directory/filename ("" for the download's own name); nil if
// the name is free.
func (s *DownloadsService) Clash(id, directory, filename string) (*Clash, error) {
	svc, err := s.app.service()
	if err != nil {
		return nil, err
	}
	return svc.Engine.Clash(id, directory, filename)
}

// Player is an installed video player that can play a download while it downloads.
type Player = platform.Player

// Players lists installed players that can play a download in progress, default first.
func (s *DownloadsService) Players() ([]Player, error) {
	svc, err := s.app.service()
	if err != nil {
		return nil, err
	}
	return svc.Players(), nil
}

// Watch opens a video while it downloads, in player (from Players) or, for "", the default
// player or another one that can play it yet.
func (s *DownloadsService) Watch(id, player string) error {
	svc, err := s.app.service()
	if err != nil {
		return err
	}
	return svc.Watch(id, player)
}

// CopyStreamAddress copies the local stream URL of a download, for players Snag doesn't know.
func (s *DownloadsService) CopyStreamAddress(id string) error {
	svc, err := s.app.service()
	if err != nil {
		return err
	}
	address, err := svc.StreamAddress(id)
	if err != nil {
		return err
	}
	if !s.app.wails.Clipboard.SetText(address) {
		return errors.New("couldn't copy the address")
	}
	return nil
}

// Open opens a finished download's file with its usual program.
func (s *DownloadsService) Open(id string) error {
	svc, err := s.app.service()
	if err != nil {
		return err
	}
	return svc.Open(id)
}

// Reveal shows a download's file in the file manager.
func (s *DownloadsService) Reveal(id string) error {
	svc, err := s.app.service()
	if err != nil {
		return err
	}
	return svc.Reveal(id)
}

// CopyLink puts a download's address on the clipboard.
func (s *DownloadsService) CopyLink(id string) error {
	svc, err := s.app.service()
	if err != nil {
		return err
	}
	d, err := svc.Engine.Get(id)
	if err != nil {
		return err
	}
	if !s.app.wails.Clipboard.SetText(d.URL) {
		return errors.New("couldn't copy the link")
	}
	return nil
}

// Check verifies a finished file against text (a pasted checksum in any common form) or, if "",
// the one Snag found. It runs in the background and reports via the download's Check.
func (s *DownloadsService) Check(id, text string) error {
	return s.withEngine(func(e *engine.Engine) error { return e.Check(id, strings.TrimSpace(text)) })
}

// Redownload downloads a finished file again, replacing it once complete.
func (s *DownloadsService) Redownload(id string) (Download, error) {
	svc, err := s.app.service()
	if err != nil {
		return Download{}, err
	}
	d, err := svc.Engine.Redownload(id)
	return viewOf(d), err
}

// CopyChecksum copies a finished file's SHA-256, hashing it unless a check already did.
func (s *DownloadsService) CopyChecksum(id string) (string, error) {
	svc, err := s.app.service()
	if err != nil {
		return "", err
	}
	d, err := svc.Engine.Get(id)
	if err != nil {
		return "", err
	}
	if d.Status != engine.StatusDone {
		return "", engine.ErrNotFinished
	}
	sum := d.Check.Got
	if d.Check.Algo != checksum.SHA256 || sum == "" {
		ctx, cancel := context.WithTimeout(context.Background(), hashTimeout)
		defer cancel()
		if sum, err = checksum.File(ctx, d.Path, checksum.SHA256, nil); err != nil {
			return "", err
		}
	}
	if !s.app.wails.Clipboard.SetText(sum) {
		return "", errors.New("couldn't copy the checksum")
	}
	return sum, nil
}

// Said is one line of a download's activity log.
type Said struct {
	At   int64  `json:"at"` // milliseconds since 1970
	Text string `json:"text"`
}

// Activity returns a download's recent log since Snag started, oldest first: program output and
// Snag's own notes.
func (s *DownloadsService) Activity(id string) ([]Said, error) {
	svc, err := s.app.service()
	if err != nil {
		return nil, err
	}
	said, err := svc.Engine.Activity(id)
	if err != nil {
		return nil, err
	}
	out := make([]Said, len(said))
	for i, line := range said {
		out[i] = Said{At: line.At.UnixMilli(), Text: line.Text}
	}
	return out, nil
}

// PauseAll pauses every running or waiting download.
func (s *DownloadsService) PauseAll() error {
	return s.withEngine(func(e *engine.Engine) error { e.PauseAll(); return nil })
}

// ResumeAll resumes every paused download and retries the failed ones.
func (s *DownloadsService) ResumeAll() error {
	return s.withEngine(func(e *engine.Engine) error { e.ResumeAll(); return nil })
}

// ClearDone takes finished downloads off the list, keeping their files.
func (s *DownloadsService) ClearDone() error {
	return s.withEngine(func(e *engine.Engine) error { e.ClearDone(); return nil })
}

func (s *DownloadsService) withEngine(do func(*engine.Engine) error) error {
	svc, err := s.app.service()
	if err != nil {
		return err
	}
	return do(svc.Engine)
}
