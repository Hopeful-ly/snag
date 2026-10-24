package naming

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Roham1299/snag/internal/meta"
)

// Template names a list's downloads from their tags, with fields in braces: "{artist} - {title}".
// An empty field drops out with its joining text and brackets: "{title} ({year})" gives "Song",
// not "Song ()".
type Template string

// Field is one of a template's fields.
type Field struct {
	Name  string `json:"name"` // without braces
	About string `json:"about"`
}

// Fields are every field a template may use.
var Fields = []Field{
	{"title", "The song's or video's title"},
	{"artist", "Its artist, or else who put it up. Featured artists are named in the title"},
	{"album", "Its album"},
	{"album_artist", "Its album's artist"},
	{"track", "Its number on the album: 07"},
	{"disc", "Its disc of the album"},
	{"year", "Its year"},
	{"show", "The show an episode is of"},
	{"season", "An episode's season: 02"},
	{"episode", "An episode's number: 05"},
	{"n", "Its place in the list: 07"},
	{"playlist", "The list's name"},
}

// FieldSet is the fields offered for each kind of list, in display order.
type FieldSet struct {
	Songs    []Field `json:"songs"`
	Videos   []Field `json:"videos"`
	Episodes []Field `json:"episodes"`
}

// Offered is the fields a window shows for each kind of list; the rest still work.
func Offered() FieldSet {
	return FieldSet{
		Songs:    pick("artist", "title", "album", "track", "year", "n", "playlist"),
		Videos:   pick("n", "title", "artist", "playlist"),
		Episodes: pick("show", "season", "episode", "title"),
	}
}

func pick(names ...string) []Field {
	out := make([]Field, 0, len(names))
	for _, name := range names {
		for _, f := range Fields {
			if f.Name == name {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// Values fill a template's fields for one download.
type Values struct {
	Tags   meta.Tags
	Number string // {n}, zero-padded
	List   string
}

func (v Values) field(name string) string {
	pad := func(n int) string {
		if n <= 0 {
			return ""
		}
		return fmt.Sprintf("%02d", n)
	}
	t := v.Tags
	artist, title := fileCredits(t)
	switch name {
	case "title":
		return title
	case "artist":
		return artist
	case "album":
		return t.Album
	case "album_artist":
		return t.AlbumArtist
	case "track":
		return pad(t.Track)
	case "disc":
		if t.Disc <= 0 {
			return ""
		}
		return strconv.Itoa(t.Disc)
	case "year":
		if len(t.Date) >= 4 {
			return t.Date[:4]
		}
		return ""
	case "show":
		return t.Show
	case "season":
		return pad(t.Season)
	case "episode":
		return pad(t.Episode)
	case "n":
		return v.Number
	case "playlist":
		return v.List
	}
	return ""
}

// featInTitle matches a title that already credits featured artists: "(feat. …)", "[featuring …]",
// "feat. …" at the end.
var featInTitle = regexp.MustCompile(`(?i)(?:\(|\[)\s*(?:featuring|feat|ft)\.?\s+|(?:^|\s)(?:featuring|feat|ft)\.?\s+\S`)

// fileCredits moves featured artists from the artist into the title, "(feat. A and B)", for the
// file name only; the tags keep every artist. The full artist list stays when the album artist is
// missing, "Various Artists", or not among the song's artists.
func fileCredits(t meta.Tags) (artist, title string) {
	artist, title = t.Artist, t.Title
	mains, all := namesOf(t.AlbumArtist), namesOf(t.Artist)
	if len(mains) == 0 || len(all) == 0 || variousArtists(mains) {
		return artist, title
	}
	main := map[string]bool{}
	for _, name := range mains {
		main[strings.ToLower(name)] = true
	}
	var featured []string
	onAlbum := false
	for _, name := range all {
		if main[strings.ToLower(name)] {
			onAlbum = true
			continue
		}
		featured = append(featured, name)
	}
	if !onAlbum || len(featured) == 0 {
		return artist, title
	}
	artist = strings.TrimSpace(t.AlbumArtist)
	title = strings.TrimSpace(title)
	if title == "" || featInTitle.MatchString(title) {
		return artist, title
	}
	return artist, title + " (feat. " + joinNames(featured) + ")"
}

// namesOf splits Tags' comma-separated names.
func namesOf(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func variousArtists(names []string) bool {
	return len(names) == 1 && strings.EqualFold(names[0], "Various Artists")
}

// joinNames writes "A", "A and B", "A, B and C".
func joinNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
	}
}

// ErrEmptyTemplate means a template names nothing.
var ErrEmptyTemplate = errors.New("write how to name the files, such as {artist} - {title}")

// parse splits t so texts[i] precedes fields[i] and the last text follows the last field.
// "{{" and "}}" are literal braces.
func (t Template) parse() (texts, fields []string, err error) {
	var text strings.Builder
	s := string(t)
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case (c == '{' || c == '}') && i+1 < len(s) && s[i+1] == c:
			text.WriteByte(c)
			i++
		case c == '{':
			end := strings.IndexByte(s[i:], '}')
			if end < 0 {
				return nil, nil, errors.New("a { has no } to close it")
			}
			name := strings.ToLower(strings.TrimSpace(s[i+1 : i+end]))
			if !known(name) {
				return nil, nil, fmt.Errorf("there's no field {%s}; there are %s", name, fieldList())
			}
			texts, fields = append(texts, text.String()), append(fields, name)
			text.Reset()
			i += end
		case c == '}':
			return nil, nil, errors.New("a } has no { before it")
		default:
			text.WriteByte(c)
		}
	}
	if len(fields) == 0 {
		return nil, nil, ErrEmptyTemplate
	}
	return append(texts, text.String()), fields, nil
}

// Check returns why t can't name files, or nil.
func (t Template) Check() error {
	_, _, err := t.parse()
	return err
}

// Name is the file name, without extension, that t gives v; "" when t is invalid or yields nothing.
func (t Template) Name(v Values) string {
	texts, fields, err := t.parse()
	if err != nil {
		return ""
	}
	values := make([]string, len(fields))
	for i, f := range fields {
		values[i] = Safe(v.field(f), "") // no path separators
	}
	shown := false // a field so far was non-empty
	for i, value := range values {
		if value != "" {
			shown = true
			continue
		}
		before, after := &texts[i], &texts[i+1]
		open := lastRune(*before)
		if shut := closing[open]; shut != 0 && strings.HasPrefix(strings.TrimLeft(*after, " "), string(shut)) {
			*before = strings.TrimSuffix(strings.TrimRight(*before, " "), string(open))
			*after = strings.TrimPrefix(strings.TrimLeft(*after, " "), string(shut))
		}
		switch {
		case shown:
			*before = ""
		case i+1 < len(fields): // leading field: drop the text joining it to the next
			*after = ""
		}
	}
	if !shown {
		return ""
	}
	var name strings.Builder
	for i, text := range texts {
		name.WriteString(text)
		if i < len(values) {
			name.WriteString(values[i])
		}
	}
	return Safe(name.String(), "")
}

var closing = map[rune]rune{'(': ')', '[': ']', '{': '}'}

func lastRune(s string) rune {
	s = strings.TrimRight(s, " ")
	if s == "" {
		return 0
	}
	return []rune(s)[len([]rune(s))-1]
}

func known(name string) bool {
	for _, f := range Fields {
		if f.Name == name {
			return true
		}
	}
	return false
}

func fieldList() string {
	names := make([]string, len(Fields))
	for i, f := range Fields {
		names[i] = "{" + f.Name + "}"
	}
	return strings.Join(names, " ")
}

// Naming is how a list's download is named once its tags are known.
type Naming struct {
	Template Template `json:"template"`
	Number   string   `json:"number,omitempty"` // for {n}
	List     string   `json:"list,omitempty"`   // for {playlist}
}

// IsSet reports whether there is a template.
func (n Naming) IsSet() bool { return n.Template != "" }

// Name is the name n gives a file tagged t; "" for none.
func (n Naming) Name(t meta.Tags) string {
	return n.Template.Name(Values{Tags: t, Number: n.Number, List: n.List})
}
