package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/Roham1299/snag/internal/config"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/playlist"
	"github.com/Roham1299/snag/internal/tools"
)

// slowYtDLP is a fake yt-dlp whose downloads never finish.
func slowYtDLP(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in yt-dlp is a shell script")
	}
	path := filepath.Join(t.TempDir(), "yt-dlp")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexec sleep 60\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

type lister struct{ list playlist.Playlist }

func (l lister) List(_ context.Context, req playlist.Request) (playlist.Playlist, error) {
	if l.list.Entries == nil {
		return playlist.Playlist{}, playlist.ErrNotAList
	}
	p := l.list
	p.Page = req.URL
	return p, nil
}

func season(n int) playlist.Playlist {
	p := playlist.Playlist{Title: "Lanterns: Season 1", Current: -1}
	for i := range n {
		p.Entries = append(p.Entries, playlist.Entry{URL: fmt.Sprintf("https://v.example/e%d", i+1), Title: fmt.Sprintf("Episode %d", i+1)})
	}
	return p
}

func TestDownloadPlaylist(t *testing.T) {
	s := startServiceWith(t, tools.Paths{YtDLP: slowYtDLP(t)})
	s.Playlists = lister{season(4)}
	p, err := s.ListPlaylist(context.Background(), playlist.Request{URL: "https://v.example/s1"})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.Playlist(p.ID); err != nil || len(got.Entries) != 4 {
		t.Fatalf("kept %+v, %v", got, err)
	}

	added, err := s.DownloadPlaylist(p.ID, playlist.Choice{Entries: []int{2, 1, 3}, Quality: "720"})
	if err != nil {
		t.Fatal(err)
	}
	if added.Added != 3 || added.Already != 0 || added.Queue.Name != "Lanterns: Season 1" || added.Queue.AtOnce != playlistAtOnce ||
		added.Queue.Naming != "{n} - {title}" {
		t.Fatalf("added %+v", added)
	}
	waitUntil(t, "the list's queue to start", func() bool {
		running := 0
		for _, d := range s.Engine.List() {
			if d.Status == engine.StatusDownloading {
				running++
			}
		}
		return running == playlistAtOnce
	})
	var urls []string
	var downloads []engine.Download
	for _, d := range s.Engine.List() {
		if d.Queue == added.Queue.ID {
			downloads = append(downloads, d)
		}
	}
	slices.SortFunc(downloads, func(a, b engine.Download) int { return int(a.Position - b.Position) })
	for _, d := range downloads {
		urls = append(urls, d.URL)
	}
	if want := []string{"https://v.example/e3", "https://v.example/e2", "https://v.example/e4"}; !slices.Equal(urls, want) {
		t.Errorf("queue order %v, want %v", urls, want)
	}
	folder := filepath.Join(s.Config().DownloadDir, "Lanterns Season 1")
	if d := downloads[0]; d.Directory != folder || d.Filename != "03 - Episode 3" || d.Label != "720p" || d.Status == engine.StatusPending {
		t.Errorf("download %+v", d)
	}
	if !slices.Contains(s.Config().RecentDirs, s.Config().DownloadDir) {
		t.Errorf("the folder isn't among the recent ones: %v", s.Config().RecentDirs)
	}

	again, err := s.DownloadPlaylist(p.ID, playlist.Choice{Entries: []int{0, 1}, Quality: "720"})
	if err != nil {
		t.Fatal(err)
	}
	if again.Queue.ID != added.Queue.ID || again.Added != 1 || again.Already != 1 {
		t.Errorf("again %+v", again)
	}
	have, err := s.PlaylistHave(p.ID, "720")
	if err != nil || !slices.Equal(have, []bool{true, true, true, true}) {
		t.Errorf("have %v, %v", have, err)
	}
	if have, _ := s.PlaylistHave(p.ID, "mp3"); slices.Contains(have, true) {
		t.Errorf("have at another quality: %v", have)
	}

	if _, err := s.DownloadPlaylist("gone", playlist.Choice{Entries: []int{0}, Quality: "720"}); !errors.Is(err, ErrNoPlaylist) {
		t.Errorf("a list not kept: %v", err)
	}
	if _, err := s.ListPlaylist(context.Background(), playlist.Request{URL: "https://v.example/one"}); err != nil {
		t.Errorf("listing again: %v", err)
	}
	s.Playlists = lister{}
	if _, err := s.ListPlaylist(context.Background(), playlist.Request{URL: "https://v.example/one"}); !errors.Is(err, playlist.ErrNotAList) {
		t.Errorf("a page of one video: %v", err)
	}
}

func TestDownloadPlaylistWithoutRoomForAQueue(t *testing.T) {
	s := startService(t)
	for i := range config.MaxQueues {
		if _, err := s.SaveQueue(config.Queue{Name: fmt.Sprint("Q", i), AtOnce: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.SaveQueue(config.Queue{Name: "One too many", AtOnce: 1}); err == nil {
		t.Error("a queue past the limit was taken")
	}
	s.Playlists = lister{season(2)}
	p, _ := s.ListPlaylist(context.Background(), playlist.Request{URL: "https://v.example/s1"})
	added, err := s.DownloadPlaylist(p.ID, playlist.Choice{Entries: []int{0}, Quality: "best"})
	if err != nil {
		t.Fatal(err)
	}
	if added.Queue.ID != config.MainQueue || added.Note == "" || added.Added != 1 {
		t.Errorf("added %+v", added)
	}
}

func TestPlaylistFolder(t *testing.T) {
	s := startService(t)
	p := season(3)
	if got := s.PlaylistFolder(p); got != filepath.Join(s.Config().DownloadDir, "Lanterns Season 1") {
		t.Errorf("list folder %q", got)
	}
	p.Entries = p.Entries[:1]
	if got := s.PlaylistFolder(p); got != s.Config().DownloadDir {
		t.Errorf("one song %q", got)
	}
}
