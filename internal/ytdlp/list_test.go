package ytdlp

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestInfoArgsListFlat(t *testing.T) {
	args := InfoArgs(InfoOptions{URL: "https://www.youtube.com/watch?v=a&list=b", Flat: true})
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--yes-playlist --flat-playlist --playlist-end 1000") || slices.Contains(args, "--no-playlist") {
		t.Errorf("flat args: %s", joined)
	}
	if args := InfoArgs(InfoOptions{URL: "https://x/v"}); !slices.Contains(args, "--no-playlist") || slices.Contains(args, "--flat-playlist") {
		t.Errorf("describing args: %v", args)
	}
}

// Trimmed yt-dlp output; the SoundCloud set's entries have no titles.
func TestReadFlatPlaylists(t *testing.T) {
	for _, c := range []struct {
		file, title, site, firstURL, firstTitle string
		duration                                float64
	}{
		{"list-youtube.json", "Popular Music Videos", "YoutubeTab", "https://www.youtube.com/watch?v=fOT0BUpITw8",
			"BELLAKEO (Video Oficial) - Peso Pluma, Anitta", 235},
		{"list-soundcloud.json", "Soulhack", "SoundcloudSet", "https://soundcloud.com/forss/city-ports", "", 0},
	} {
		data, err := os.ReadFile("testdata/" + c.file)
		if err != nil {
			t.Fatal(err)
		}
		info, err := readInfo(data)
		if err != nil {
			t.Fatal(err)
		}
		if info.Type != "playlist" || info.Title != c.title || info.Site != c.site || len(info.Entries) != 3 {
			t.Fatalf("%s: %s %q %s with %d entries", c.file, info.Type, info.Title, info.Site, len(info.Entries))
		}
		first := info.Entries[0]
		if first.URL != c.firstURL || first.Title != c.firstTitle || first.Duration != c.duration || first.ID == "" {
			t.Errorf("%s: first entry %+v", c.file, first)
		}
	}
}

func TestEntryFor(t *testing.T) {
	list := Info{Type: "playlist", Entries: []Info{
		{ID: "aaa", Format: Format{URL: "https://www.youtube.com/watch?v=aaa"}},
		{ID: "bbb", Format: Format{URL: "https://www.youtube.com/watch?v=bbb"}},
		{ID: "291", Format: Format{URL: "https://soundcloud.com/forss/soulhack"}},
	}}
	for page, want := range map[string]int{
		"https://www.youtube.com/watch?v=bbb&list=PL1&index=2":         1,
		"https://www.youtube.com/shorts/aaa":                           0,
		"https://soundcloud.com/forss/soulhack?in=forss/sets/soulhack": 2,
		"https://www.youtube.com/playlist?list=PL1":                    -1,
		"https://soundcloud.com/forss/sets/soulhack":                   -1,
		"not a page \x7f": -1,
	} {
		if got := list.EntryFor(page); got != want {
			t.Errorf("EntryFor(%q) = %d, want %d", page, got, want)
		}
	}
	if (Info{Type: "playlist", Entries: list.Entries[:1]}).IsList() || !list.IsList() {
		t.Error("IsList")
	}
}

func TestListAddress(t *testing.T) {
	for page, want := range map[string]string{
		"https://www.youtube.com/watch?v=ECORZn-W9f0&list=PLabc":   "https://www.youtube.com/playlist?list=PLabc",
		"https://youtu.be/ECORZn-W9f0?list=PLabc":                  "https://www.youtube.com/playlist?list=PLabc",
		"https://music.youtube.com/watch?v=x&list=OLAK5uy_abc":     "https://music.youtube.com/playlist?list=OLAK5uy_abc",
		"https://www.youtube.com/watch?v=x&list=RDx":               "https://www.youtube.com/watch?v=x&list=RDx", // a mix
		"https://www.youtube.com/playlist?list=PLabc":              "https://www.youtube.com/playlist?list=PLabc",
		"https://www.youtube.com/@YouTube/videos":                  "https://www.youtube.com/@YouTube/videos",
		"https://www.youtube.com/@veritasium":                      "https://www.youtube.com/@veritasium/videos",
		"https://www.youtube.com/@veritasium/":                     "https://www.youtube.com/@veritasium/videos",
		"https://www.youtube.com/@veritasium/featured":             "https://www.youtube.com/@veritasium/videos",
		"https://m.youtube.com/@veritasium?si=abc":                 "https://m.youtube.com/@veritasium/videos",
		"https://www.youtube.com/channel/UCHnyfMqiRRG1u-2MsSQLbXA": "https://www.youtube.com/channel/UCHnyfMqiRRG1u-2MsSQLbXA/videos",
		"https://www.youtube.com/c/veritasium":                     "https://www.youtube.com/c/veritasium/videos",
		"https://www.youtube.com/user/1veritasium":                 "https://www.youtube.com/user/1veritasium/videos",
		"https://www.youtube.com/@veritasium/shorts":               "https://www.youtube.com/@veritasium/shorts",
		"https://www.youtube.com/@veritasium/playlists":            "https://www.youtube.com/@veritasium/playlists",
		"https://soundcloud.com/forss/sets/soulhack":               "https://soundcloud.com/forss/sets/soulhack",
		"https://www.notyoutube.com/watch?v=x&list=PLabc":          "https://www.notyoutube.com/watch?v=x&list=PLabc",
	} {
		if got := listAddress(page); got != want {
			t.Errorf("listAddress(%s) = %s, want %s", page, got, want)
		}
	}
}
