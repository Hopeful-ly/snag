package playlist

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Roham1299/snag/internal/meta"
	"github.com/Roham1299/snag/internal/spotify"
)

// Spotify lists Spotify playlists, albums, artists' top songs, Liked Songs and single songs,
// fetching the audio from YouTube Music (ViaSearch). Without the open page's session only the
// embed is read: a playlist's first 100 songs without albums, and no Liked Songs.
type Spotify struct {
	Client spotify.Client
}

// List implements Lister.
func (l Spotify) List(ctx context.Context, req Request) (Playlist, error) {
	ref, ok := spotify.Parse(req.URL)
	if !ok {
		return Playlist{}, ErrNotAList
	}
	list, err := l.Client.Read(ctx, ref, req.Spotify)
	if errors.Is(err, spotify.ErrNotFound) || errors.Is(err, spotify.ErrSignIn) {
		return Playlist{}, err
	}
	if err != nil {
		return Playlist{}, fmt.Errorf("couldn't read %s from Spotify: %w", ref.Kind.Noun(), err)
	}
	if len(list.Songs) == 0 {
		return Playlist{}, fmt.Errorf("%s has no songs Snag can find", ref.Kind.Noun())
	}
	p := Playlist{
		Title:   list.Title,
		Site:    "Spotify",
		Page:    ref.URL(),
		Current: -1,
		Via:     ViaSearch,
	}
	switch ref.Kind {
	case spotify.Track:
		p.Title = strings.TrimSpace(list.Owner + " - " + list.Title)
	case spotify.Artist:
		p.Title = list.Title + " - Top songs"
	}
	if list.Partial {
		p.Note = fmt.Sprintf("Spotify shows %d of its songs to Snag on its own. For all of them, open the "+
			"playlist in your browser and use the Snag button on the page.", len(list.Songs))
	}
	for i, s := range list.Songs {
		p.Entries = append(p.Entries, songEntry(list, i, s))
	}
	return p, nil
}

// songEntry titles a song "Artist - Title" in a playlist and just "Title" on its own album.
func songEntry(list spotify.List, i int, s spotify.Song) Entry {
	page := spotify.Ref{Kind: spotify.Track, ID: s.ID}.URL()
	artists := strings.Join(s.Artists, ", ")
	title := s.Title
	if list.Kind != spotify.Album && artists != "" {
		title = artists + " - " + s.Title
	}
	e := Entry{
		URL:      page,
		Title:    title,
		Duration: s.Duration.Seconds(),
		Meta: meta.Label{Tags: meta.Tags{
			Title: s.Title, Artist: artists, Album: s.Album, AlbumArtist: strings.Join(s.AlbumArtists, ", "),
			Track: s.Number, Disc: s.Disc, Date: s.Date, Cover: s.Cover, Page: page,
		}},
	}
	if list.Kind != spotify.Track && list.Kind != spotify.Artist {
		// Defaults only: YouTube Music's album, if found, wins.
		e.Meta.Defaults = meta.Tags{Album: list.Title, Track: i + 1}
	}
	return e
}
