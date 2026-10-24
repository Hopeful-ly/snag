package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Roham1299/snag/internal/applemusic"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/meta"
	"github.com/Roham1299/snag/internal/pagedl"
	"github.com/Roham1299/snag/internal/spotify"
	"github.com/Roham1299/snag/internal/ytmusic"
)

// findSong finds a Spotify song on YouTube Music, re-reading it from Spotify because a playlist
// read from its embed has no cover or date. fallback is searched when YouTube Music has no match.
func (s *Service) findSong(ctx context.Context, ref spotify.Ref, fallback string) (pagedl.Source, error) {
	list, err := s.spotify.Read(ctx, ref, nil)
	if err != nil {
		return pagedl.Source{}, fmt.Errorf("couldn't read the song from Spotify: %w", err)
	}
	song := list.Songs[0]
	want := ytmusic.Song{Title: song.Title, Artists: song.Artists, Album: song.Album, Duration: song.Duration}
	return s.foundSong(ctx, want, meta.Tags{
		Title: song.Title, Artist: strings.Join(song.Artists, ", "), Cover: song.Cover, Date: song.Date,
	}, fallback)
}

// findAppleSong finds an Apple Music song on YouTube Music, tagged as Apple Music has it.
func (s *Service) findAppleSong(ctx context.Context, ref applemusic.Ref, fallback string) (pagedl.Source, error) {
	song, err := s.apple.Song(ctx, ref)
	if err != nil {
		return pagedl.Source{}, err
	}
	want := ytmusic.Song{Title: song.Title, Artists: song.Artists, Album: song.Album, Duration: song.Duration}
	return s.foundSong(ctx, want, meta.Tags{
		Title: song.Title, Artist: strings.Join(song.Artists, ", "), Album: song.Album,
		AlbumArtist: strings.Join(song.AlbumArtists, ", "), Track: song.Number, Disc: song.Disc,
		Date: song.Date, Cover: song.Cover,
	}, fallback)
}

// foundSong searches YouTube Music, then fallback if set; with no match it returns
// engine.NeedsPlace so the user is asked where to look.
func (s *Service) foundSong(ctx context.Context, want ytmusic.Song, tags meta.Tags, fallback string) (pagedl.Source, error) {
	address, err := s.songs.Find(ctx, want)
	if errors.Is(err, ytmusic.ErrNoMatch) && fallback != "" {
		found, _, ferr := s.songs.FindOn(ctx, want, fallback)
		if ferr == nil {
			address, err = found, nil
		} else if !errors.Is(ferr, ytmusic.ErrNoMatch) {
			return pagedl.Source{}, ferr
		}
	}
	if errors.Is(err, ytmusic.ErrNoMatch) {
		return pagedl.Source{}, &engine.NeedsPlace{Text: err.Error()}
	}
	if err != nil {
		return pagedl.Source{}, err
	}
	return pagedl.Source{URL: address, Tags: tags}, nil
}
