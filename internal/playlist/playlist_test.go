package playlist

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/meta"
	"github.com/Roham1299/snag/internal/probe"
	"github.com/Roham1299/snag/internal/tools"
)

var season = Playlist{
	Title: "Lanterns Season 1",
	Page:  "https://v.example/watch?v=b&list=s1",
	Entries: []Entry{
		{URL: "https://v.example/watch?v=a", Title: "Pilot", Duration: 60},
		{URL: "https://v.example/watch?v=b", Title: "The Lamp: Part 2/3"},
		{URL: "https://v.example/watch?v=c"},
	},
	Current: 1,
	Headers: map[string]string{"Referer": "https://v.example/"},
}

func TestPlan(t *testing.T) {
	reqs, err := Plan(season, Choice{Entries: []int{1, 2, 1}, Quality: "720", Directory: "/d", Queue: "q1", Naming: "{n} - {title}"})
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 2 {
		t.Fatalf("got %d downloads, want 2 (one entry twice): %+v", len(reqs), reqs)
	}
	first := reqs[0]
	if first.URL != season.Entries[1].URL || first.Page != first.URL || first.Kind != engine.KindStream ||
		first.Label != "720p" || first.Directory != "/d" || first.Queue != "q1" || !first.Confirmed ||
		first.Headers["Referer"] != "https://v.example/" {
		t.Errorf("first download %+v", first)
	}
	if first.Filename != "02 - The Lamp Part 2 3" {
		t.Errorf("name %q", first.Filename)
	}
	if first.Format != "bv*[height<=720]+ba/b[height<=720]/bv*+ba/b" {
		t.Errorf("format %q", first.Format)
	}
	if reqs[1].Filename != "03" || reqs[1].Title != "" {
		t.Errorf("an entry without a title is still numbered, got %q / %q", reqs[1].Filename, reqs[1].Title)
	}

	mp3, _ := Plan(season, Choice{Entries: []int{0}, Quality: "mp3", Naming: "{artist} - {title}"})
	if mp3[0].Audio != "mp3" || mp3[0].Label != "Audio MP3" {
		t.Errorf("mp3 download %+v", mp3[0])
	}
	for _, bad := range []Choice{{Entries: []int{0}, Quality: "999"}, {Quality: "best"}, {Entries: []int{3}, Quality: "best"}} {
		if _, err := Plan(season, bad); err == nil {
			t.Errorf("Plan(%+v) took it", bad)
		}
	}
}

func TestPlanPadsNumbers(t *testing.T) {
	long := Playlist{Entries: make([]Entry, 120)}
	for i := range long.Entries {
		long.Entries[i] = Entry{URL: "https://v.example/" + string(rune('a'+i%26)), Title: "Song"}
	}
	reqs, _ := Plan(long, Choice{Entries: []int{6, 119}, Quality: "best", Naming: "{n} - {title}"})
	if reqs[0].Filename != "007 - Song" || reqs[1].Filename != "120 - Song" {
		t.Errorf("names %q, %q", reqs[0].Filename, reqs[1].Filename)
	}
}

func TestDefaultQuality(t *testing.T) {
	for page, want := range map[string]string{
		"https://soundcloud.com/forss/sets/soulhack":      "mp3",
		"https://music.youtube.com/playlist?list=OLAK5uy": "mp3",
		"https://artist.bandcamp.com/album/x":             "mp3",
		"https://www.youtube.com/playlist?list=PL1":       "best",
		"https://notsoundcloud.com/sets/x":                "best",
	} {
		if got := DefaultQuality(Playlist{Page: page}); got != want {
			t.Errorf("DefaultQuality(%s) = %s, want %s", page, got, want)
		}
	}
}

func TestKindOf(t *testing.T) {
	show := Playlist{Entries: []Entry{{Meta: meta.Label{Only: true, Tags: meta.Tags{Episode: 1}}}}}
	if k := KindOf(show, "best"); k != KindEpisodes {
		t.Errorf("a show: %q", k)
	}
	if k := KindOf(season, "mp3"); k != KindSongs {
		t.Errorf("audio: %q", k)
	}
	if k := KindOf(season, "720"); k != KindVideos {
		t.Errorf("video: %q", k)
	}
}

func TestNamed(t *testing.T) {
	p := Playlist{Entries: []Entry{
		{Title: "S1 E1 · Pilot", Meta: meta.Label{Only: true, Tags: meta.Tags{
			Title: "Pilot", Show: "Lanterns", Season: 1, Episode: 1, Page: "https://x", Cover: "https://c",
		}}},
		{Title: "YUNGBLUD - Zombie", Meta: meta.Label{Tags: meta.Tags{Title: "Zombie", Artist: "YUNGBLUD"}}},
	}}
	got := p.Named()
	if got.Entries[0].Tags != (meta.Tags{Title: "Pilot", Show: "Lanterns", Season: 1, Episode: 1}) {
		t.Errorf("episode tags %+v", got.Entries[0].Tags)
	}
	if got.Entries[1].Tags.Title != "Zombie" || got.Entries[1].Tags.Artist != "YUNGBLUD" {
		t.Errorf("song tags %+v", got.Entries[1].Tags)
	}
	if p.Entries[0].Tags.Show != "" {
		t.Error("naming the copy changed the list")
	}
}

func TestStore(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := &Store{now: func() time.Time { return now }}
	kept := s.Put(season)
	if kept.ID == "" || kept.Title != season.Title {
		t.Fatalf("kept %+v", kept)
	}
	if got, ok := s.Get(kept.ID); !ok || got.ID != kept.ID {
		t.Errorf("Get = %+v, %v", got, ok)
	}
	for range keepLists {
		s.Put(season)
	}
	if _, ok := s.Get(kept.ID); ok {
		t.Error("the oldest list is still kept past the limit")
	}
	last := s.Put(season)
	now = now.Add(keepFor + time.Minute)
	if _, ok := s.Get(last.ID); ok {
		t.Error("a list is still kept after an hour")
	}
}

func TestYtDLPLister(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in yt-dlp is a shell script")
	}
	dir := t.TempDir()
	list := `{"_type": "playlist", "title": " Season 1 ", "extractor_key": "Example", "entries": [
		{"id": "x", "title": "Gone"},
		{"id": "a", "url": "https://v.example/watch?v=a", "title": "One", "duration": 61},
		{"id": "b", "url": "https://v.example/watch?v=b"}]}`
	if err := os.WriteFile(filepath.Join(dir, "list.json"), []byte(list), 0o644); err != nil {
		t.Fatal(err)
	}
	ytdlp := filepath.Join(dir, "yt-dlp")
	if err := os.WriteFile(ytdlp, []byte("#!/bin/sh\ncat "+filepath.Join(dir, "list.json")+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	lister := YtDLP{Prober: probe.New(tools.Paths{YtDLP: ytdlp})}
	got, err := lister.List(context.Background(), Request{URL: "https://v.example/watch?v=b&list=s1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Season 1" || got.Site != "Example" || len(got.Entries) != 2 || got.Current != 1 ||
		got.Entries[0] != (Entry{URL: "https://v.example/watch?v=a", Title: "One", Duration: 61,
			Meta: meta.Label{Defaults: meta.Tags{Album: "Season 1", Track: 1}}}) || got.Entries[1].Meta.Defaults.Track != 2 {
		t.Errorf("list %+v", got)
	}
}

func TestPlanEpisodes(t *testing.T) {
	p := Playlist{Title: "Lanterns", Via: ViaBrowser, Entries: []Entry{
		{URL: "https://site.example/tv/1/1/1", Title: "S1 E1 · Pilot",
			Meta: meta.Label{Only: true, Tags: meta.Tags{Title: "Pilot", Show: "Lanterns", Season: 1, Episode: 1}}},
		{URL: "https://site.example/tv/1/1/2", Title: "S1 E2",
			Meta: meta.Label{Only: true, Tags: meta.Tags{Title: "Episode 2", Show: "Lanterns", Season: 1, Episode: 2}}},
	}}
	reqs, err := Plan(p, Choice{Entries: []int{0, 1}, Quality: "best", Naming: "{show} S{season}E{episode} - {title}"})
	if err != nil {
		t.Fatal(err)
	}
	if reqs[0].Filename != "Lanterns S01E01 - Pilot" || !reqs[0].Resolve || reqs[1].Filename != "Lanterns S01E02 - Episode 2" ||
		reqs[0].Meta.Tags.Title != "Pilot" {
		t.Errorf("planned %+v", reqs)
	}
}
