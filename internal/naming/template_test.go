package naming

import (
	"strings"
	"testing"

	"github.com/Roham1299/snag/internal/meta"
)

func TestTemplateName(t *testing.T) {
	song := Values{
		Tags:   meta.Tags{Title: "Get Lucky", Artist: "Daft Punk", Album: "Random Access Memories", Track: 8, Date: "2013-05-20"},
		Number: "07", List: "Liked Songs",
	}
	bare := Values{Tags: meta.Tags{Title: "Get Lucky"}, Number: "07"}
	episode := Values{Tags: meta.Tags{Title: "Pilot", Show: "Breaking Bad", Season: 1, Episode: 1}}
	for _, c := range []struct {
		template Template
		v        Values
		want     string
	}{
		{"{artist} - {title}", song, "Daft Punk - Get Lucky"},
		{"{n} - {artist} - {title}", song, "07 - Daft Punk - Get Lucky"},
		{"{track}. {title} ({year})", song, "08. Get Lucky (2013)"},
		{"{playlist} {n}", song, "Liked Songs 07"},
		{"{show} S{season}E{episode} - {title}", episode, "Breaking Bad S01E01 - Pilot"},
		// A field with nothing drops out with what joins it.
		{"{artist} - {title}", bare, "Get Lucky"},
		{"{n} - {artist} - {title}", bare, "07 - Get Lucky"},
		{"{title} ({year})", bare, "Get Lucky"},
		{"{title} [{album}] - {n}", bare, "Get Lucky - 07"},
		{"({year}) {title}", bare, "Get Lucky"},
		{"{artist} - {album} - {title}", bare, "Get Lucky"},
		{"{artist}", bare, ""},
		// Braces as they are, and what no file name may have.
		{"{{{title}}}", bare, "{Get Lucky}"},
		{"{TITLE}", Values{Tags: meta.Tags{Title: "AC/DC: Live?"}}, "AC DC Live"},
	} {
		if got := c.template.Name(c.v); got != c.want {
			t.Errorf("%q gives %q, want %q", c.template, got, c.want)
		}
	}
}

func TestFeaturedArtists(t *testing.T) {
	name := func(tags meta.Tags) string {
		return Template("{artist} - {title}").Name(Values{Tags: tags})
	}
	for _, c := range []struct {
		tags meta.Tags
		want string
	}{
		// The title already names them, so the file name keeps that and drops them from the artist.
		{meta.Tags{
			Title:  "Get Lucky (feat. Pharrell Williams and Nile Rodgers)",
			Artist: "Daft Punk, Pharrell Williams, Nile Rodgers", AlbumArtist: "Daft Punk",
		}, "Daft Punk - Get Lucky (feat. Pharrell Williams and Nile Rodgers)"},
		{meta.Tags{
			Title: "Get Lucky (ft. Pharrell Williams)", Artist: "Daft Punk, Pharrell Williams", AlbumArtist: "Daft Punk",
		}, "Daft Punk - Get Lucky (ft. Pharrell Williams)"},
		// A plain title gains the feat tag. Two names use "and"; three use a comma.
		{meta.Tags{
			Title: "Get Lucky", Artist: "Daft Punk, Pharrell Williams", AlbumArtist: "Daft Punk",
		}, "Daft Punk - Get Lucky (feat. Pharrell Williams)"},
		{meta.Tags{
			Title: "Get Lucky", Artist: "Daft Punk, Pharrell Williams, Nile Rodgers", AlbumArtist: "Daft Punk",
		}, "Daft Punk - Get Lucky (feat. Pharrell Williams and Nile Rodgers)"},
		{meta.Tags{
			Title: "Song", Artist: "A, B, C, D", AlbumArtist: "A",
		}, "A - Song (feat. B, C and D)"},
		// Everyone on the song is an album artist: a collaboration, left as the artist list.
		{meta.Tags{Title: "One More Time", Artist: "Daft Punk", AlbumArtist: "Daft Punk"}, "Daft Punk - One More Time"},
		{meta.Tags{Title: "Team", Artist: "A, B", AlbumArtist: "B, A"}, "A, B - Team"},
		// Nothing to split on.
		{meta.Tags{Title: "Get Lucky", Artist: "Daft Punk, Pharrell Williams"}, "Daft Punk, Pharrell Williams - Get Lucky"},
		{meta.Tags{
			Title: "Song", Artist: "Daft Punk, Pharrell Williams", AlbumArtist: "Various Artists",
		}, "Daft Punk, Pharrell Williams - Song"},
		{meta.Tags{Title: "Song", Artist: "Daft Punk", AlbumArtist: "Hans Zimmer"}, "Daft Punk - Song"},
	} {
		if got := name(c.tags); got != c.want {
			t.Errorf("%q / %q gives %q, want %q", c.tags.Artist, c.tags.Title, got, c.want)
		}
	}
}

func TestOfferedFields(t *testing.T) {
	got := Offered()
	names := func(fields []Field) string {
		var s string
		for _, f := range fields {
			s += f.Name + " "
		}
		return s
	}
	if names(got.Songs) != "artist title album track year n playlist " {
		t.Errorf("songs: %s", names(got.Songs))
	}
	if names(got.Videos) != "n title artist playlist " {
		t.Errorf("videos: %s", names(got.Videos))
	}
	if names(got.Episodes) != "show season episode title " {
		t.Errorf("episodes: %s", names(got.Episodes))
	}
}

func TestTemplateCheck(t *testing.T) {
	for template, want := range map[Template]string{
		"{artist} - {title}": "",
		"{{title}}":          "write how",
		"":                   "write how",
		"just text":          "write how",
		"{artst} - {title}":  "no field {artst}",
		"{title":             "no }",
		"title}":             "no {",
	} {
		err := template.Check()
		if (want == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), want)) {
			t.Errorf("Check(%q) = %v, want %q", template, err, want)
		}
	}
}
