package app

import (
	"github.com/Roham1299/snag/internal/config"
	"github.com/Roham1299/snag/internal/naming"
	"github.com/Roham1299/snag/internal/playlist"
	"github.com/Roham1299/snag/internal/service"
)

// Playlist is a list of videos or tracks found on a page.
type Playlist = playlist.Playlist

// Quality is how every entry of a list is downloaded.
type Quality = playlist.Quality

// PlaylistPick is what to download of a list and how; EveryHours > 0 also follows it.
type PlaylistPick struct {
	playlist.Choice
	Follow     bool `json:"follow"`
	EveryHours int  `json:"everyHours"`
}

// PlaylistAdded reports the result of downloading a list.
type PlaylistAdded = service.PlaylistAdded

// PlaylistsService backs the "Download playlist" window.
type PlaylistsService struct {
	app *App
}

// PlaylistView is a list as its window shows it, with its defaults.
type PlaylistView struct {
	Playlist
	Folder string `json:"folder"`
	// FolderName is the list's own subfolder, already the last part of Folder; "" for a
	// single song. It is appended when a recent place is picked.
	FolderName string            `json:"folderName,omitempty"`
	Naming     naming.Template   `json:"naming"`
	Kind       playlist.NameKind `json:"kind"`
	Defaults   config.Naming     `json:"defaults"`
	Fields     naming.FieldSet   `json:"fields"`
	Quality    string            `json:"quality"`
	Qualities  []Quality         `json:"qualities"`
	// CanFollow is false for streaming sites' shows; Following is nil when not followed.
	CanFollow bool          `json:"canFollow"`
	Following *Subscription `json:"following"`
}

// Get returns the list with id.
func (s *PlaylistsService) Get(id string) (PlaylistView, error) {
	svc, err := s.app.service()
	if err != nil {
		return PlaylistView{}, err
	}
	p, err := svc.Playlist(id)
	if err != nil {
		return PlaylistView{}, err
	}
	quality := playlist.DefaultQuality(p)
	folderName := ""
	if len(p.Entries) != 1 {
		folderName = service.PlaylistSubfolder(p)
	}
	var following *Subscription
	if sub, ok := svc.Following(p.Page); ok {
		view := subscriptionView(sub, false)
		following = &view
	}
	return PlaylistView{
		CanFollow:  p.Via != playlist.ViaBrowser,
		Following:  following,
		Playlist:   p.Named(),
		Folder:     svc.PlaylistFolder(p),
		FolderName: folderName,
		Naming:     svc.PlaylistNaming(p, quality, ""),
		Kind:       playlist.KindOf(p, quality),
		Defaults:   svc.Config().Naming,
		Fields:     naming.Offered(),
		Quality:    quality,
		Qualities:  playlist.QualitiesFor(p),
	}, nil
}

// Have reports, per entry, whether Snag has it at quality already.
func (s *PlaylistsService) Have(id, quality string) ([]bool, error) {
	svc, err := s.app.service()
	if err != nil {
		return nil, err
	}
	return svc.PlaylistHave(id, quality)
}

// Download downloads the picked entries, then closes the list's window and shows the main one,
// unless there is a note for the list's window to show.
func (s *PlaylistsService) Download(id string, pick PlaylistPick) (PlaylistAdded, error) {
	svc, err := s.app.service()
	if err != nil {
		return PlaylistAdded{}, err
	}
	var added PlaylistAdded
	if pick.Follow {
		added, err = svc.FollowPlaylist(id, pick.Choice, pick.EveryHours)
	} else {
		added, err = svc.DownloadPlaylist(id, pick.Choice)
	}
	if err != nil || added.Note != "" {
		return added, err
	}
	s.app.playlists.close(id)
	s.app.show()
	return added, nil
}
