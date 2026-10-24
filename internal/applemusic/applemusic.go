// Package applemusic reads song lists from Apple Music, for finding each song elsewhere.
// Apple Music's sound is DRM-protected and is never fetched.
package applemusic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Kind is the kind of page an Apple Music address points at.
type Kind string

// Kinds of Apple Music page.
const (
	Album    Kind = "album"
	Track    Kind = "song" // the path segment
	Playlist Kind = "playlist"
	Artist   Kind = "artist" // their top songs
)

// Noun names the kind for use in a sentence.
func (k Kind) Noun() string {
	switch k {
	case Track:
		return "the song"
	case Artist:
		return "the artist's top songs"
	}
	return "the " + string(k)
}

// Ref names an Apple Music page. Song is set for a /song/ page or an album's ?i= track.
type Ref struct {
	Kind       Kind
	ID         string
	Song       string
	Storefront string
	Address    string // the page, without a tracking query
}

// One reports whether ref is a single song.
func (r Ref) One() bool { return r.Song != "" }

// URL is the address of its page.
func (r Ref) URL() string { return r.Address }

var (
	catalogPath = regexp.MustCompile(`^/(?:([a-z]{2})/)?(album|song|playlist|artist)/[^/]+/([^/]+)/?$`)
	playlistID  = regexp.MustCompile(`^pl\.[0-9a-fA-F]+$`)
	numericID   = regexp.MustCompile(`^[0-9]{2,}$`)
	songCount   = regexp.MustCompile(`^(\d+) songs\b`)
	isoDate     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)
	yearOnly    = regexp.MustCompile(`^(19|20)\d{2}$`)
)

// Parse reads an Apple Music address and reports whether it is one.
func Parse(address string) (Ref, bool) {
	u, err := url.Parse(strings.TrimSpace(address))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || !appleHost(u.Hostname()) {
		return Ref{}, false
	}
	m := catalogPath.FindStringSubmatch(u.Path)
	if m == nil {
		return Ref{}, false
	}
	kind := Kind(m[2])
	id := m[3]
	song := ""
	switch kind {
	case Playlist:
		if !playlistID.MatchString(id) {
			return Ref{}, false
		}
	case Album:
		if !numericID.MatchString(id) {
			return Ref{}, false
		}
		if i := u.Query().Get("i"); i != "" {
			if !numericID.MatchString(i) {
				return Ref{}, false
			}
			song = i
		}
	default:
		if !numericID.MatchString(id) {
			return Ref{}, false
		}
		if kind == Track {
			song = id
		}
	}
	u.Host = "music.apple.com"
	u.Fragment = ""
	q := url.Values{}
	if song != "" && kind == Album {
		q.Set("i", song)
	}
	u.RawQuery = q.Encode()
	return Ref{Kind: kind, ID: id, Song: song, Storefront: m[1], Address: u.String()}, true
}

func appleHost(host string) bool {
	return host == "music.apple.com" || host == "geo.music.apple.com" || host == "embed.music.apple.com"
}

// Song is one track as Apple Music names it.
type Song struct {
	ID, Title    string
	Artists      []string
	Album        string
	AlbumArtists []string
	Number, Disc int
	Date         string // YYYY-MM-DD, or YYYY alone
	Cover, URL   string
	Duration     time.Duration
}

// List is an album, a playlist, an artist's top songs, or a song alone.
type List struct {
	Title, Owner string
	Kind         Kind
	Songs        []Song
	// Total is the page's song count, set only when it lists fewer than that.
	Total int
}

// Client reads Apple Music's public pages, and iTunes' lookup for what a page leaves out.
type Client struct {
	HTTP   *http.Client // nil for a 20 s timeout
	Page   string       // test override for the page host
	Lookup string       // test override for the lookup URL
}

// ErrNotFound means Apple Music has no such page, or no such song on it.
var ErrNotFound = errors.New("there's nothing on Apple Music at that address")

var defaultHTTP = &http.Client{Timeout: 20 * time.Second}

const (
	lookupBase = "https://itunes.apple.com/lookup"
	pageAgent  = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

func (c Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return defaultHTTP
}

// Read reads ref's list from its public page.
func (c Client) Read(ctx context.Context, ref Ref) (List, error) {
	body, err := c.get(ctx, c.pageURL(ref.Address))
	if err != nil {
		return List{}, err
	}
	list, err := parse(body, ref)
	if err != nil {
		return List{}, err
	}
	if ref.Kind == Album && ref.Song == "" {
		if err := c.fillAlbum(ctx, ref.ID, list.Songs); err != nil {
			return List{}, err
		}
	}
	if ref.One() {
		for i := range list.Songs {
			extra, err := c.lookupSong(ctx, list.Songs[i].ID)
			if err != nil {
				if ctx.Err() != nil {
					return List{}, ctx.Err()
				}
				continue
			}
			merge(&list.Songs[i], extra)
		}
	}
	return list, nil
}

// Song reads one song from iTunes' lookup.
func (c Client) Song(ctx context.Context, ref Ref) (Song, error) {
	id := ref.Song
	if id == "" {
		id = ref.ID
	}
	song, err := c.lookupSong(ctx, id)
	if err != nil {
		return Song{}, fmt.Errorf("couldn't read the song from Apple Music: %w", err)
	}
	return song, nil
}

func (c Client) pageURL(address string) string {
	if c.Page == "" {
		return address
	}
	u, err := url.Parse(address)
	base, berr := url.Parse(c.Page)
	if err != nil || berr != nil {
		return address
	}
	u.Scheme, u.Host = base.Scheme, base.Host
	return u.String()
}

func (c Client) get(ctx context.Context, address string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", pageAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	res, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Apple Music answered %s", res.Status) //nolint:staticcheck // a name
	}
	return io.ReadAll(io.LimitReader(res.Body, 4<<20))
}

func (c Client) lookupURL(id, entity string) string {
	base := c.Lookup
	if base == "" {
		base = lookupBase
	}
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	q := u.Query()
	q.Set("id", id)
	if entity != "" {
		q.Set("entity", entity)
		q.Set("limit", "200")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// fillAlbum fills disc numbers and other gaps from an album lookup. Only cancellation is an error.
func (c Client) fillAlbum(ctx context.Context, id string, songs []Song) error {
	tracks, err := c.lookupTracks(ctx, c.lookupURL(id, "song"))
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return nil
	}
	byID := map[string]Song{}
	for _, t := range tracks {
		byID[t.ID] = t
	}
	for i := range songs {
		if extra, ok := byID[songs[i].ID]; ok {
			merge(&songs[i], extra)
		}
	}
	return nil
}

func (c Client) lookupSong(ctx context.Context, id string) (Song, error) {
	if !numericID.MatchString(id) {
		return Song{}, ErrNotFound
	}
	tracks, err := c.lookupTracks(ctx, c.lookupURL(id, ""))
	if err != nil {
		return Song{}, err
	}
	for _, t := range tracks {
		if t.ID == id {
			return t, nil
		}
	}
	if len(tracks) == 1 {
		return tracks[0], nil
	}
	return Song{}, ErrNotFound
}

func (c Client) lookupTracks(ctx context.Context, address string) ([]Song, error) {
	body, err := c.get(ctx, address)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Results []struct {
			Wrapper string `json:"wrapperType"`
			Name    string `json:"trackName"`
			Artist  string `json:"artistName"`
			Album   string `json:"collectionName"`
			AlbumBy string `json:"collectionArtistName"`
			ID      int64  `json:"trackId"`
			Number  int    `json:"trackNumber"`
			Disc    int    `json:"discNumber"`
			Date    string `json:"releaseDate"`
			Millis  int    `json:"trackTimeMillis"`
			Art     string `json:"artworkUrl100"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("couldn't read Apple Music's catalog: %w", err)
	}
	var songs []Song
	for _, r := range doc.Results {
		if r.Wrapper != "track" || r.Name == "" || r.ID == 0 {
			continue
		}
		songs = append(songs, Song{
			ID: idOf(r.ID), Title: r.Name, Artists: splitArtists(r.Artist),
			Album: cleanAlbum(r.Album), AlbumArtists: splitArtists(r.AlbumBy),
			Number: r.Number, Disc: r.Disc, Date: dateOf(r.Date),
			Cover: coverSize(r.Art), Duration: time.Duration(r.Millis) * time.Millisecond,
		})
	}
	return songs, nil
}

func idOf(id int64) string { return strconv.FormatInt(id, 10) }

// merge fills what the page left empty from a lookup of the same song.
func merge(dst *Song, src Song) {
	if dst.Title == "" {
		dst.Title = src.Title
	}
	if len(dst.Artists) == 0 {
		dst.Artists = src.Artists
	}
	if dst.Album == "" {
		dst.Album = src.Album
	}
	if len(dst.AlbumArtists) == 0 {
		dst.AlbumArtists = src.AlbumArtists
	}
	if dst.Number == 0 {
		dst.Number = src.Number
	}
	if dst.Disc == 0 {
		dst.Disc = src.Disc
	}
	// A year alone is what a page's header says; the lookup has the day.
	if dst.Date == "" || (len(dst.Date) == 4 && len(src.Date) > 4) {
		dst.Date = src.Date
	}
	if dst.Cover == "" {
		dst.Cover = src.Cover
	}
	if dst.Duration == 0 {
		dst.Duration = src.Duration
	}
	if dst.URL == "" {
		dst.URL = src.URL
	}
}

type pageDoc struct {
	Data []struct {
		Data struct {
			Sections []section `json:"sections"`
		} `json:"data"`
	} `json:"data"`
}

type section struct {
	ItemKind string            `json:"itemKind"`
	Items    []json.RawMessage `json:"items"`
}

type lockup struct {
	Title           string      `json:"title"`
	ArtistName      string      `json:"artistName"`
	Artists         string      `json:"artists"`
	Album           string      `json:"album"`
	ReleaseDate     string      `json:"releaseDate"`
	TrackNumber     *int        `json:"trackNumber"`
	Duration        int         `json:"duration"`
	SubtitleLinks   []namedLink `json:"subtitleLinks"`
	TertiaryLinks   []namedLink `json:"tertiaryLinks"`
	ArtistLinks     []namedLink `json:"artistLinks"`
	QuaternaryTitle string      `json:"quaternaryTitle"`
	Description     string      `json:"description"`
	Artwork         artBox      `json:"artwork"`
	Content         content     `json:"contentDescriptor"`
	Tracks          []lockup    `json:"tracks"`
}

type namedLink struct {
	Title string `json:"title"`
	Segue struct {
		Destination struct {
			Content content `json:"contentDescriptor"`
		} `json:"destination"`
	} `json:"segue"`
}

type content struct {
	Kind        string `json:"kind"`
	Identifiers struct {
		ID adamID `json:"storeAdamID"`
	} `json:"identifiers"`
	URL string `json:"url"`
}

type artBox struct {
	Dictionary struct {
		URL string `json:"url"`
	} `json:"dictionary"`
}

// adamID is a store id, which the page sends as a string and sometimes a number.
type adamID string

func (a *adamID) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) >= 2 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*a = adamID(s)
		return nil
	}
	if bytes.Equal(b, []byte("null")) {
		return nil
	}
	*a = adamID(b)
	return nil
}

func (a artBox) url() string { return coverTemplate(a.Dictionary.URL) }

func serverData(body []byte) ([]byte, error) {
	const marker = `id="serialized-server-data">`
	i := bytes.Index(body, []byte(marker))
	if i < 0 {
		return nil, errors.New("Apple Music's page has no catalog") //nolint:staticcheck // a name
	}
	rest := body[i+len(marker):]
	end := bytes.Index(rest, []byte("</script>"))
	if end < 0 {
		return nil, errors.New("Apple Music's page has no catalog") //nolint:staticcheck // a name
	}
	return rest[:end], nil
}

type header struct {
	title, date, cover string
	artists            []string
	total              int
	playlist           bool
}

func parse(body []byte, ref Ref) (List, error) {
	raw, err := serverData(body)
	if err != nil {
		return List{}, err
	}
	var doc pageDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return List{}, fmt.Errorf("couldn't read Apple Music's catalog: %w", err)
	}
	if len(doc.Data) == 0 {
		return List{}, ErrNotFound
	}
	var head header
	var tracks []lockup
	head.playlist = ref.Kind == Playlist
	for _, sec := range doc.Data[0].Data.Sections {
		for _, item := range sec.Items {
			var row lockup
			if json.Unmarshal(item, &row) != nil {
				continue
			}
			switch sec.ItemKind {
			case "containerDetailHeaderLockup":
				if head.title == "" {
					head.title = cleanAlbum(row.Title)
					head.artists = linkTitles(row.SubtitleLinks, "artist")
					head.date = dateOf(row.QuaternaryTitle)
					if !head.playlist {
						head.cover = row.Artwork.url()
					}
				}
			case "artistDetailHeader":
				if head.title == "" {
					head.title = strings.TrimSpace(row.Title)
					if head.title != "" {
						head.artists = []string{head.title}
					}
				}
			case "trackLockup":
				if row.Title != "" {
					tracks = append(tracks, row)
				}
			case "artistFeaturedContentAndTracks":
				tracks = append(tracks, row.Tracks...)
			case "containerDetailTracklistFooterLockup":
				if m := songCount.FindStringSubmatch(row.Description); m != nil {
					head.total, _ = strconv.Atoi(m[1])
				}
			case "songDetailHeader":
				return songList(row, ref)
			}
		}
	}
	if len(tracks) == 0 {
		return List{}, fmt.Errorf("%s has no songs Snag can find", ref.Kind.Noun())
	}
	from := fromPage{album: head.title, date: head.date, cover: head.cover, artists: head.artists, storefront: ref.Storefront}
	if head.playlist {
		from.album, from.date, from.cover, from.artists = "", "", "", nil
	}
	songs := make([]Song, 0, len(tracks))
	for _, row := range tracks {
		song := row.song(from)
		if song.Title == "" {
			continue
		}
		if ref.Song != "" && song.ID != ref.Song {
			continue
		}
		songs = append(songs, song)
	}
	if len(songs) == 0 {
		return List{}, ErrNotFound
	}
	list := List{Title: head.title, Kind: ref.Kind, Songs: songs}
	if !head.playlist && len(head.artists) > 0 {
		list.Owner = strings.Join(head.artists, ", ")
	}
	if ref.One() {
		list.Kind = Track
		list.Title = songs[0].Title
		list.Owner = strings.Join(songs[0].Artists, ", ")
		return list, nil
	}
	if head.total > len(songs) {
		list.Total = head.total
	}
	return list, nil
}

func songList(row lockup, ref Ref) (List, error) {
	song := row.song(fromPage{storefront: ref.Storefront})
	if song.Title == "" {
		return List{}, ErrNotFound
	}
	if song.ID == "" {
		song.ID = ref.ID
	}
	if song.URL == "" {
		song.URL = ref.Address
	}
	return List{
		Title: song.Title, Owner: strings.Join(song.Artists, ", "), Kind: Track, Songs: []Song{song},
	}, nil
}

// fromPage is the album (or artist) a track is listed on, for what the track itself doesn't say.
type fromPage struct {
	album, date, cover, storefront string
	artists                        []string
}

func (row lockup) song(from fromPage) Song {
	artists := linkTitles(row.SubtitleLinks, "artist")
	if len(artists) == 0 {
		artists = linkTitles(row.ArtistLinks, "artist")
	}
	if len(artists) == 0 && row.ArtistName != "" {
		artists = splitArtists(row.ArtistName)
	}
	if len(artists) == 0 && row.Artists != "" {
		artists = splitArtists(row.Artists)
	}
	album := firstLink(row.TertiaryLinks, "album")
	if album == "" {
		album = cleanAlbum(row.Album)
	}
	date := ""
	if album == "" {
		for _, l := range row.SubtitleLinks {
			if linkKind(l) == "artist" || !strings.Contains(l.Title, "·") {
				continue
			}
			album, date = albumYear(l.Title)
			break
		}
	}
	if d := dateOf(row.ReleaseDate); d != "" {
		date = d
	}
	if album == "" {
		album = from.album
	}
	if date == "" {
		date = from.date
	}
	if len(artists) == 0 {
		artists = from.artists
	}
	cover := row.Artwork.url()
	if cover == "" {
		cover = from.cover
	}
	albumArtists := from.artists
	if len(albumArtists) == 0 {
		albumArtists = linkTitles(row.ArtistLinks, "artist")
	}
	id := string(row.Content.Identifiers.ID)
	page := row.Content.URL
	if page == "" && id != "" {
		sf := from.storefront
		if sf == "" {
			sf = "us"
		}
		page = "https://music.apple.com/" + sf + "/song/song/" + id
	}
	number := 0
	if row.TrackNumber != nil {
		number = *row.TrackNumber
	}
	return Song{
		ID: id, Title: strings.TrimSpace(row.Title), Artists: artists, Album: album, AlbumArtists: albumArtists,
		Number: number, Date: date, Cover: cover, URL: page,
		Duration: time.Duration(row.Duration) * time.Millisecond,
	}
}

func linkKind(l namedLink) string { return l.Segue.Destination.Content.Kind }

func linkTitles(links []namedLink, kind string) []string {
	var out []string
	for _, l := range links {
		if linkKind(l) == kind && strings.TrimSpace(l.Title) != "" {
			out = append(out, strings.TrimSpace(l.Title))
		}
	}
	return out
}

func firstLink(links []namedLink, kind string) string {
	names := linkTitles(links, kind)
	if len(names) == 0 {
		return ""
	}
	return cleanAlbum(names[0])
}

func splitArtists(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	var out []string
	for _, piece := range strings.Split(name, ", ") {
		for _, one := range strings.Split(piece, " & ") {
			if one = strings.TrimSpace(one); one != "" {
				out = append(out, one)
			}
		}
	}
	return out
}

func cleanAlbum(name string) string {
	name = strings.TrimSpace(name)
	for _, suffix := range []string{" - Single", " - EP"} {
		name = strings.TrimSuffix(name, suffix)
	}
	return strings.TrimSpace(name)
}

func albumYear(s string) (string, string) {
	parts := strings.Split(s, "·")
	if len(parts) != 2 {
		return cleanAlbum(s), ""
	}
	return cleanAlbum(parts[0]), yearOf(parts[1])
}

func dateOf(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "\u202f", " "), "\u00a0", " "))
	if s == "" {
		return ""
	}
	if t, err := time.Parse("January 2, 2006", s); err == nil {
		return t.Format("2006-01-02")
	}
	if t, err := time.Parse("2 January 2006", s); err == nil {
		return t.Format("2006-01-02")
	}
	if isoDate.MatchString(s) {
		return s[:10]
	}
	return yearOf(s)
}

func yearOf(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\u202f", " "))
	parts := strings.Split(s, "·")
	last := strings.TrimSpace(parts[len(parts)-1])
	if yearOnly.MatchString(last) {
		return last
	}
	return ""
}

// coverTemplate is a page's artwork URL, whose size is a placeholder, at a size worth embedding.
func coverTemplate(address string) string {
	if address == "" {
		return ""
	}
	address = strings.ReplaceAll(address, "{w}", "1400")
	address = strings.ReplaceAll(address, "{h}", "1400")
	address = strings.ReplaceAll(address, "{f}", "jpg")
	return address
}

// coverSize is a lookup artwork URL, whose "100x100bb" is the small one, at a size worth embedding.
func coverSize(address string) string {
	return strings.Replace(address, "100x100", "1400x1400", 1)
}
