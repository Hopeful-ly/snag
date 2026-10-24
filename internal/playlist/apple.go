package playlist

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Roham1299/snag/internal/applemusic"
	"github.com/Roham1299/snag/internal/meta"
)

// AppleMusic lists Apple Music albums, playlists, artists' top songs and single songs. The audio
// is DRM-protected, so each song is fetched from YouTube Music (ViaSearch) and tagged with Apple
// Music's metadata.
type AppleMusic struct {
	Client applemusic.Client
}

// List implements Lister.
func (l AppleMusic) List(ctx context.Context, req Request) (Playlist, error) {
	ref, ok := applemusic.Parse(req.URL)
	if !ok {
		return Playlist{}, ErrNotAList
	}
	list, err := l.Client.Read(ctx, ref)
	if errors.Is(err, applemusic.ErrNotFound) {
		return Playlist{}, err
	}
	if err != nil {
		return Playlist{}, fmt.Errorf("couldn't read %s from Apple Music: %w", ref.Kind.Noun(), err)
	}
	if len(list.Songs) == 0 {
		return Playlist{}, fmt.Errorf("%s has no songs Snag can find", ref.Kind.Noun())
	}
	p := Playlist{
		Title:   list.Title,
		Site:    "Apple Music",
		Page:    ref.URL(),
		Current: -1,
		Via:     ViaSearch,
	}
	switch list.Kind {
	case applemusic.Track:
		p.Title = strings.TrimSpace(list.Owner + " - " + list.Title)
	case applemusic.Artist:
		p.Title = list.Title + " - Top songs"
	}
	if list.Total > len(list.Songs) {
		p.Note = fmt.Sprintf("Apple Music shows %d of its %d songs on this page.", len(list.Songs), list.Total)
	}
	for i, s := range list.Songs {
		p.Entries = append(p.Entries, appleEntry(list, i, s))
	}
	return p, nil
}

func appleEntry(list applemusic.List, i int, s applemusic.Song) Entry {
	artists := strings.Join(s.Artists, ", ")
	title := s.Title
	if list.Kind != applemusic.Album && artists != "" {
		title = artists + " - " + s.Title
	}
	e := Entry{
		URL:      s.URL,
		Title:    title,
		Duration: s.Duration.Seconds(),
		Meta: meta.Label{Tags: meta.Tags{
			Title: s.Title, Artist: artists, Album: s.Album, AlbumArtist: strings.Join(s.AlbumArtists, ", "),
			Track: s.Number, Disc: s.Disc, Date: s.Date, Cover: s.Cover, Page: s.URL,
		}},
	}
	if list.Kind != applemusic.Track && list.Kind != applemusic.Artist {
		e.Meta.Defaults = meta.Tags{Album: list.Title, Track: i + 1}
	}
	return e
}
