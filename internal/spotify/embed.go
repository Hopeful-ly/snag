package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var nextData = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`)

const maxPage = 4 << 20

type embedEntity struct {
	Name     string `json:"name"`
	Subtitle string `json:"subtitle"` // playlist owner or album artists
	Artists  []struct {
		Name string `json:"name"`
	} `json:"artists"`
	CoverArt struct {
		Sources []picture `json:"sources"`
	} `json:"coverArt"`
	VisualIdentity struct {
		Image []struct {
			URL       string `json:"url"`
			MaxWidth  int    `json:"maxWidth"`
			MaxHeight int    `json:"maxHeight"`
		} `json:"image"`
	} `json:"visualIdentity"`
	ReleaseDate *struct {
		ISO string `json:"isoString"`
	} `json:"releaseDate"`
	Duration  float64 `json:"duration"` // ms
	TrackList []struct {
		URI      string  `json:"uri"`
		Title    string  `json:"title"`
		Subtitle string  `json:"subtitle"` // artists
		Duration float64 `json:"duration"` // ms
	} `json:"trackList"`
}

const embedLimit = 100

// Embed reads ref from its embed page, which needs no account. Playlists stop at 100 songs, and
// playlist and artist songs have no album.
func (c Client) Embed(ctx context.Context, ref Ref) (List, error) {
	base := c.Open
	if base == "" {
		base = OpenBase
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/embed/"+string(ref.Kind)+"/"+ref.ID, nil)
	if err != nil {
		return List{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Safari/537.36")
	resp, err := c.do(req)
	if err != nil {
		if errors.Is(err, errSession) {
			return List{}, ErrNotFound
		}
		return List{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPage))
	if err != nil {
		return List{}, err
	}
	ent, err := parseEmbed(body)
	if err != nil {
		return List{}, err
	}
	return ent.list(ref), nil
}

func parseEmbed(page []byte) (embedEntity, error) {
	m := nextData.FindSubmatch(page)
	if m == nil {
		return embedEntity{}, errors.New("the Spotify page has changed: Snag can't read it")
	}
	var data struct {
		Props struct {
			PageProps struct {
				State struct {
					Data struct {
						Entity *embedEntity `json:"entity"`
					} `json:"data"`
				} `json:"state"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(m[1], &data); err != nil {
		return embedEntity{}, err
	}
	ent := data.Props.PageProps.State.Data.Entity
	if ent == nil {
		return embedEntity{}, ErrNotFound
	}
	return *ent, nil
}

func (e embedEntity) cover() string {
	if c := largest(e.CoverArt.Sources); c != "" {
		return c
	}
	var sources []picture
	for _, p := range e.VisualIdentity.Image {
		sources = append(sources, picture{URL: p.URL, Width: p.MaxWidth, Height: p.MaxHeight})
	}
	return largest(sources)
}

func (e embedEntity) list(ref Ref) List {
	l := List{Ref: ref, Title: strings.TrimSpace(e.Name), Owner: strings.TrimSpace(e.Subtitle), Cover: e.cover()}
	date := ""
	if e.ReleaseDate != nil {
		date = day(e.ReleaseDate.ISO, "DAY")
	}
	if ref.Kind == Track {
		var artists []string
		for _, a := range e.Artists {
			artists = append(artists, a.Name)
		}
		l.Owner = strings.Join(artists, ", ")
		l.Songs = []Song{{ID: ref.ID, Title: l.Title, Artists: artists, Cover: l.Cover, Date: date,
			Duration: time.Duration(e.Duration) * time.Millisecond}}
		return l
	}
	if ref.Kind == Artist {
		l.Owner = l.Title // the subtitle says "Top tracks"
	}
	for i, t := range e.TrackList {
		s := Song{ID: idOf(t.URI), Title: strings.TrimSpace(t.Title), Artists: splitArtists(t.Subtitle),
			Duration: time.Duration(t.Duration) * time.Millisecond}
		if ref.Kind == Album {
			s.Album, s.AlbumArtists, s.Cover, s.Date, s.Number = l.Title, splitArtists(e.Subtitle), l.Cover, date, i+1
		}
		l.Songs = append(l.Songs, s)
	}
	l.Partial = ref.Kind == Playlist && len(e.TrackList) >= embedLimit
	return l
}

// splitArtists splits on a comma and no-break space, which embeds join artists with; a plain
// ", " stays, keeping "Tyler, The Creator" one artist.
func splitArtists(s string) []string {
	var artists []string
	for _, a := range strings.Split(s, ", ") {
		if a = strings.TrimSpace(a); a != "" {
			artists = append(artists, a)
		}
	}
	return artists
}
