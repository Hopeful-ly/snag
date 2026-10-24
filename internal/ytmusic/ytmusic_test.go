package ytmusic

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// YouTube Music's first three songs for "Limp Bizkit Behind Blue Eyes", as found.
var behindBlueEyes = []Result{
	{Title: "Behind Blue Eyes", Track: "Behind Blue Eyes", Duration: 270, Artists: []string{"Limp Bizkit"},
		Album: "Results May Vary", WebpageURL: "https://music.youtube.com/watch?v=ozllPih3hOk"},
	{Title: "Behind Blue Eyes (Originally Performed By Limp Bizkit) (Full Vocal Version)", Duration: 271,
		Artists: []string{"Chart Collective"}, Album: "Karaoke Rock Hits, Vol. 11",
		WebpageURL: "https://music.youtube.com/watch?v=k8CoKXMuhME"},
	{Title: "Behind Blue Eyes", Duration: 273, Artists: []string{"The Dark Anger"}, Album: "Justice Dawns (2016)",
		WebpageURL: "https://music.youtube.com/watch?v=o2wphzswtxc"},
}

func TestPick(t *testing.T) {
	song := Song{Title: "Behind Blue Eyes", Artists: []string{"Limp Bizkit"}, Album: "Results May Vary",
		Duration: 269973 * time.Millisecond}
	best, ok := Pick(song, behindBlueEyes)
	if !ok || !strings.HasSuffix(best.WebpageURL, "ozllPih3hOk") {
		t.Errorf("picked %+v", best)
	}
	// Without the song itself, neither the karaoke track nor another band's song will do.
	if best, ok := Pick(song, behindBlueEyes[1:]); ok {
		t.Errorf("picked %+v", best)
	}
}

func TestScore(t *testing.T) {
	lucky := Song{Title: "Get Lucky (feat. Pharrell Williams and Nile Rodgers)",
		Artists: []string{"Daft Punk", "Pharrell Williams", "Nile Rodgers"}, Duration: 369626 * time.Millisecond}
	for _, c := range []struct {
		name string
		r    Result
		ok   bool
	}{
		{"the song", Result{Track: "Get Lucky (feat. Pharrell Williams and Nile Rodgers)", Title: "Get Lucky",
			Artists: []string{"Daft Punk"}, Duration: 369}, true},
		{"a music video named so", Result{Title: "Daft Punk - Get Lucky (Official Audio) ft. Pharrell Williams, Nile Rodgers",
			Channel: "Daft Punk", Duration: 368}, true},
		{"the radio edit", Result{Title: "Get Lucky (Radio Edit)", Artists: []string{"Daft Punk"}, Duration: 248}, false},
		{"a cover", Result{Title: "Get Lucky (Cover)", Artists: []string{"Daft Punk Tribute"}, Duration: 369}, false},
		{"a live take", Result{Title: "Get Lucky - Live", Artists: []string{"Daft Punk"}, Duration: 371}, false},
		{"another artist's song", Result{Title: "Get Lucky", Artists: []string{"Someone"}, Duration: 369}, false},
		{"another of its songs", Result{Title: "Lose Yourself to Dance", Artists: []string{"Daft Punk"}, Duration: 369}, false},
	} {
		if _, ok := Score(lucky, c.r); ok != c.ok {
			t.Errorf("%s: ok = %v", c.name, ok)
		}
	}

	// A live song is found as one.
	live := Song{Title: "Hotel California - Live", Artists: []string{"Eagles"}}
	if _, ok := Score(live, Result{Title: "Hotel California (Live)", Artists: []string{"Eagles"}}); !ok {
		t.Error("a live song isn't its live take")
	}
	// Names aren't punctuation: "Don’t" is "Don't".
	if _, ok := Score(Song{Title: "Don’t Stop Me Now", Artists: []string{"Queen"}},
		Result{Title: "Don't Stop Me Now", Artists: []string{"Queen"}}); !ok {
		t.Error("an apostrophe stops a match")
	}
}

func TestFind(t *testing.T) {
	var asked []string
	f := Finder{Search: func(_ context.Context, query string, n int) ([]Result, error) {
		asked = append(asked, query)
		if strings.HasPrefix(query, "https://music.youtube.com/search?q=Limp+Bizkit+Behind+Blue+Eyes#songs") {
			return behindBlueEyes[1:], nil // YouTube Music without it
		}
		return behindBlueEyes[:1], nil
	}}
	song := Song{Title: "Behind Blue Eyes", Artists: []string{"Limp Bizkit"}, Duration: 270 * time.Second}
	address, err := f.Find(context.Background(), song)
	if err != nil || !strings.HasSuffix(address, "ozllPih3hOk") || len(asked) != 2 ||
		asked[1] != "ytsearch3:Limp Bizkit - Behind Blue Eyes" {
		t.Errorf("Find = %q, %v after %q", address, err, asked)
	}

	none := Finder{Search: func(context.Context, string, int) ([]Result, error) { return nil, nil }}
	_, err = none.Find(context.Background(), song)
	if !errors.Is(err, ErrNoMatch) || !strings.Contains(err.Error(), "“Behind Blue Eyes” by Limp Bizkit") {
		t.Errorf("no match: %v", err)
	}
}

// A place the user names is searched on its own, and its results stay when none is the song.
func TestFindOn(t *testing.T) {
	var asked string
	var n int
	f := Finder{Search: func(_ context.Context, query string, got int) ([]Result, error) {
		asked, n = query, got
		return []Result{{Title: "Bitab (Live)", Artists: []string{"Mehrad Hidden"}, Duration: 270,
			WebpageURL: "https://www.youtube.com/watch?v=live"}}, nil
	}}
	song := Song{Title: "Bitab", Artists: []string{"Mehrad Hidden", "Saman Wilson"}, Album: "Bozorg, Vol. 2",
		Duration: 200 * time.Second}
	address, results, err := f.FindOn(context.Background(), song, PlaceYouTube)
	if !errors.Is(err, ErrNoMatch) || address != "" || len(results) != 1 || n != elsewhere ||
		asked != "ytsearch8:Mehrad Hidden, Saman Wilson - Bitab" {
		t.Errorf("FindOn = %q, %v, %d results (n %d), asked %q", address, err, len(results), n, asked)
	}

	f.Search = func(context.Context, string, int) ([]Result, error) { return behindBlueEyes[:1], nil }
	address, _, err = f.FindOn(context.Background(), Song{Title: "Behind Blue Eyes", Artists: []string{"Limp Bizkit"},
		Duration: 270 * time.Second}, PlaceSoundCloud)
	if err != nil || !strings.HasSuffix(address, "ozllPih3hOk") {
		t.Errorf("a match on SoundCloud: %q, %v", address, err)
	}
}
