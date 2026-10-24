package service

import (
	"errors"

	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/platform"
)

var errNotStarted = errors.New("it hasn't started yet; try again in a moment")

// errNeedsStreamPlayer: separate video and audio streams have no single file to open until joined.
var errNeedsStreamPlayer = errors.New("watching this while it downloads needs a video player that plays " +
	"streams, such as VLC (videolan.org); it opens normally once it has finished")

// Players lists installed video players that can play a download in progress, default first.
func (s *Service) Players() []platform.Player {
	return platform.StreamPlayers("video.mp4")
}

// Watch opens download id in player (from Players, or "" for the first that plays streams) while
// it downloads, resuming it if paused. With no such player it opens the partial file.
func (s *Service) Watch(id, player string) error {
	d, err := s.Engine.Get(id)
	if err != nil {
		return err
	}
	if d.Status == engine.StatusPaused {
		if err := s.Engine.Resume(id); err != nil {
			return err
		}
	}
	stream := d.Kind == engine.KindStream
	if s.watch != nil && (stream || (d.Size > 0 && !d.Estimated)) {
		address := s.watch.URL(d)
		if player != "" {
			return platform.PlayStreamWith(player, address)
		}
		err := platform.PlayStream(address, d.Name())
		if !errors.Is(err, platform.ErrNoStreamPlayer) {
			return err
		}
		if others := platform.StreamPlayers(d.Name()); len(others) > 0 {
			return platform.PlayStreamWith(others[0].ID, address)
		}
	}
	if stream {
		if a, err := s.Engine.Available(id, 0); err == nil && a.Separate {
			return errNeedsStreamPlayer
		}
	}
	if d.Path == "" {
		return errNotStarted
	}
	return platform.Open(d.Path)
}

// StreamAddress is the local URL any player can stream download id from while Snag runs.
func (s *Service) StreamAddress(id string) (string, error) {
	d, err := s.Engine.Get(id)
	if err != nil {
		return "", err
	}
	if s.watch == nil {
		return "", errors.New("streaming isn't available")
	}
	return s.watch.URL(d), nil
}
