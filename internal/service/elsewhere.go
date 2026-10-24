package service

import (
	"cmp"
	"context"
	"errors"
	"strings"

	"github.com/Roham1299/snag/internal/applemusic"
	"github.com/Roham1299/snag/internal/config"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/spotify"
	"github.com/Roham1299/snag/internal/ytmusic"
)

// Found is one result of looking for a song somewhere other than YouTube Music.
type Found struct {
	Title   string `json:"title"`
	By      string `json:"by"` // its artists, or the channel when it names none
	Seconds int    `json:"seconds"`
	URL     string `json:"url"`
	// Match means it passed the usual matching checks.
	Match bool `json:"match"`
}

// SearchElsewhere searches place (ytmusic.PlaceYouTube or PlaceSoundCloud) for a song YouTube
// Music had no match for.
func (s *Service) SearchElsewhere(ctx context.Context, id, place string) ([]Found, error) {
	d, err := s.Engine.Get(id)
	if err != nil {
		return nil, err
	}
	if !waitingForPlace(d) {
		return nil, errors.New("this one isn't waiting for somewhere to look")
	}
	song, err := s.songFor(ctx, d)
	if err != nil {
		return nil, err
	}
	_, results, err := s.songs.FindOn(ctx, song, place)
	if err != nil && !errors.Is(err, ytmusic.ErrNoMatch) {
		return nil, err
	}
	found := make([]Found, 0, len(results))
	for _, r := range results {
		title := cmp.Or(r.Track, r.Title)
		if title == "" || strings.TrimSpace(r.WebpageURL) == "" {
			continue
		}
		by := r.Channel
		if len(r.Artists) > 0 {
			by = strings.Join(r.Artists, ", ")
		}
		_, match := ytmusic.Score(song, r)
		found = append(found, Found{
			Title: title, By: by, Seconds: int(r.Duration), URL: r.WebpageURL, Match: match,
		})
	}
	return found, nil
}

// UseFound points the download at the address the user picked and queues it. With remember,
// place becomes its queue's fallback for unmatched songs.
func (s *Service) UseFound(id, address, place string, remember bool) error {
	if err := s.Engine.UseFound(id, address); err != nil {
		return err
	}
	if !remember || (place != ytmusic.PlaceYouTube && place != ytmusic.PlaceSoundCloud) {
		return nil
	}
	d, err := s.Engine.Get(id)
	if err != nil {
		return err
	}
	_, err = s.Update(func(c *config.Config) {
		q, ok := c.Queue(d.Queue)
		if !ok {
			return
		}
		q.Fallback = place
		c.SetQueue(q)
	})
	return err
}

// waitingForPlace checks the error text too, for downloads saved before the Where flag existed.
func waitingForPlace(d engine.Download) bool {
	return d.Status == engine.StatusError && (d.Where || strings.HasSuffix(d.Error, "no match"))
}

// songFor prefers Spotify's or Apple Music's account of the song over the download's tags.
func (s *Service) songFor(ctx context.Context, d engine.Download) (ytmusic.Song, error) {
	for _, page := range []string{d.URL, d.Page} {
		if ref, ok := spotify.Parse(page); ok && ref.Kind == spotify.Track {
			list, err := s.spotify.Read(ctx, ref, nil)
			if err != nil || len(list.Songs) == 0 {
				break
			}
			song := list.Songs[0]
			return ytmusic.Song{
				Title: song.Title, Artists: song.Artists, Album: song.Album, Duration: song.Duration,
			}, nil
		}
		if ref, ok := applemusic.Parse(page); ok && ref.One() {
			song, err := s.apple.Song(ctx, ref)
			if err != nil || song.Title == "" {
				break
			}
			return ytmusic.Song{
				Title: song.Title, Artists: song.Artists, Album: song.Album, Duration: song.Duration,
			}, nil
		}
	}
	tags := d.Meta.Known(d.Title)
	if tags.Title == "" {
		return ytmusic.Song{}, errors.New("nothing is known of this song")
	}
	var artists []string
	if tags.Artist != "" {
		artists = strings.Split(tags.Artist, ", ")
	}
	return ytmusic.Song{Title: tags.Title, Artists: artists, Album: tags.Album}, nil
}
