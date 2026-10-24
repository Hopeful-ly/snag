package app

import (
	"net/url"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// newPlaylistWindows makes the "Download playlist" windows, one per list.
func newPlaylistWindows(a *App) *dialogs {
	return newDialogs(a, func(id string) application.WebviewWindowOptions {
		title := "Download playlist"
		if svc, err := a.service(); err == nil {
			if p, err := svc.Playlist(id); err == nil && p.Title != "" {
				title += ": " + p.Title
			}
		}
		return application.WebviewWindowOptions{
			Name:      "playlist-" + id,
			Title:     title,
			Width:     640,
			Height:    680,
			MinWidth:  480,
			MinHeight: 420,
			URL:       "/?playlist=" + url.QueryEscape(id),
		}
	})
}

// ShowPlaylist opens the "Download playlist" window for a list.
func (a *App) ShowPlaylist(id string) {
	a.playlists.show(id)
}
