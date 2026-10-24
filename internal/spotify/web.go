package spotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Web player queries, and page sizes.
const (
	playlistQuery = "fetchPlaylist"
	albumQuery    = "getAlbum"
	webPage       = 100
	maxSongs      = 10000 // a playlist's limit
	maxAnswer     = 16 << 20
)

// Liked Songs are a playlist whose URI is the same for everyone and answers with the asker's
// own songs (fetchPlaylistContents); older players read the library (fetchLibraryTracks).
const (
	likedURI     = "spotify:playlist:37i9dQZF1F5p3rmiWPIYgZ"
	contentQuery = "fetchPlaylistContents"
	libraryQuery = "fetchLibraryTracks"
	libraryPage  = 50
)

// Web reads every song of a playlist, album or Liked Songs through the web player's API.
func (c Client) Web(ctx context.Context, ref Ref, s Session) (List, error) {
	switch ref.Kind {
	case Playlist:
		return c.webPlaylist(ctx, ref, ref.uri(), s)
	case Album:
		return c.webAlbum(ctx, ref, s)
	case Liked:
		return c.webLiked(ctx, s)
	}
	return List{}, fmt.Errorf("%s isn't read through the web player's API", ref.Kind.Noun())
}

// sameQuery lists aliases sharing a query's hash. fetchPlaylistMetadata is asked on every page,
// so it is captured even when fetchPlaylist isn't.
var sameQuery = map[string][]string{
	playlistQuery: {"fetchPlaylistMetadata"},
}

// pick is the first of names, or an alias, that s has a hash for; "" for none.
func (s Session) pick(names ...string) string {
	for _, n := range names {
		for _, asked := range append([]string{n}, sameQuery[n]...) {
			if s.Hashes[asked] != "" {
				return asked
			}
		}
	}
	return ""
}

func (c Client) query(ctx context.Context, s Session, name string, variables map[string]any, out any) error {
	hash := s.Hashes[name]
	if hash == "" {
		return fmt.Errorf("%w: it never asked for %s", errSession, name)
	}
	body, err := json.Marshal(map[string]any{
		"variables":     variables,
		"operationName": name,
		"extensions":    map[string]any{"persistedQuery": map[string]any{"version": 1, "sha256Hash": hash}},
	})
	if err != nil {
		return err
	}
	base := c.API
	if base == "" {
		base = APIBase
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/pathfinder/v2/query", bytes.NewReader(body))
	if err != nil {
		return err
	}
	for name, value := range map[string]string{
		"Authorization": s.Token, "Client-Token": s.ClientToken, "Spotify-App-Version": s.AppVersion,
		"User-Agent": s.UserAgent, "App-Platform": "WebPlayer",
		"Content-Type": "application/json;charset=UTF-8", "Accept": "application/json",
	} {
		if value != "" {
			req.Header.Set(name, value)
		}
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswer))
	if err != nil {
		return err
	}
	var answer struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		return fmt.Errorf("reading Spotify's answer: %w", err)
	}
	if len(answer.Errors) > 0 {
		// Mostly a query whose hash the player no longer uses.
		return fmt.Errorf("%w: %s", errSession, answer.Errors[0].Message)
	}
	return json.Unmarshal(answer.Data, out)
}

type webArtists struct {
	Items []struct {
		Profile struct {
			Name string `json:"name"`
		} `json:"profile"`
	} `json:"items"`
}

func (a webArtists) names() []string {
	var names []string
	for _, i := range a.Items {
		if n := strings.TrimSpace(i.Profile.Name); n != "" {
			names = append(names, n)
		}
	}
	return names
}

type webDate struct {
	ISO       string `json:"isoString"`
	Precision string `json:"precision"`
}

type webCover struct {
	Sources []picture `json:"sources"`
}

type webTrack struct {
	Typename string     `json:"__typename"`
	URI      string     `json:"uri"`
	Name     string     `json:"name"`
	Artists  webArtists `json:"artists"`
	Album    struct {
		Name     string     `json:"name"`
		Artists  webArtists `json:"artists"`
		CoverArt webCover   `json:"coverArt"`
		Date     webDate    `json:"date"`
	} `json:"albumOfTrack"`
	Disc     int         `json:"discNumber"`
	Number   int         `json:"trackNumber"`
	Duration webDuration `json:"trackDuration"`
	Length   webDuration `json:"duration"` // the library query's name
}

type webDuration struct {
	MS float64 `json:"totalMilliseconds"`
}

// isSong excludes podcast episodes and local files.
func (t webTrack) isSong() bool {
	return t.Typename == "Track"
}

func (t webTrack) song() Song {
	ms := t.Duration.MS
	if ms == 0 {
		ms = t.Length.MS
	}
	return Song{
		ID: idOf(t.URI), Title: strings.TrimSpace(t.Name), Artists: t.Artists.names(),
		Album: strings.TrimSpace(t.Album.Name), AlbumArtists: t.Album.Artists.names(),
		Cover: largest(t.Album.CoverArt.Sources), Date: day(t.Album.Date.ISO, t.Album.Date.Precision),
		Disc: t.Disc, Number: t.Number, Duration: time.Duration(ms) * time.Millisecond,
	}
}

// webPlaylist reads the playlist at uri. fetchPlaylistContents leaves out the name: see Client.named.
func (c Client) webPlaylist(ctx context.Context, ref Ref, uri string, s Session) (List, error) {
	name := s.pick(playlistQuery, contentQuery)
	if name == "" {
		return List{}, fmt.Errorf("%w: it never asked for a playlist", errSession)
	}
	l := List{Ref: ref}
	for offset := 0; offset < maxSongs; offset += webPage {
		var data struct {
			Playlist *struct {
				Typename string `json:"__typename"`
				Name     string `json:"name"`
				Owner    struct {
					Data struct {
						Name string `json:"name"`
					} `json:"data"`
				} `json:"ownerV2"`
				Images struct {
					Items []webCover `json:"items"`
				} `json:"images"`
				Content struct {
					Total int `json:"totalCount"`
					Items []struct {
						Item struct {
							Data webTrack `json:"data"`
						} `json:"itemV2"`
					} `json:"items"`
				} `json:"content"`
			} `json:"playlistV2"`
		}
		variables := map[string]any{"uri": uri, "offset": offset, "limit": webPage}
		if name != contentQuery {
			variables["enableWatchFeedEntrypoint"] = false
			variables["includeEpisodeContentRatingsV2"] = false
		}
		if err := c.query(ctx, s, name, variables, &data); err != nil {
			return List{}, err
		}
		p := data.Playlist
		if p == nil || (p.Typename != "" && p.Typename != "Playlist") {
			return List{}, ErrNotFound
		}
		if offset == 0 {
			l.Title, l.Owner = strings.TrimSpace(p.Name), strings.TrimSpace(p.Owner.Data.Name)
			if len(p.Images.Items) > 0 {
				l.Cover = largest(p.Images.Items[0].Sources)
			}
		}
		for _, item := range p.Content.Items {
			if t := item.Item.Data; t.isSong() {
				l.Songs = append(l.Songs, t.song())
			}
		}
		if len(p.Content.Items) < webPage || offset+webPage >= p.Content.Total {
			break
		}
	}
	return l, nil
}

func (c Client) webLiked(ctx context.Context, s Session) (List, error) {
	if s.pick(libraryQuery) != "" {
		if l, err := c.webLibrary(ctx, s); err == nil || ctx.Err() != nil {
			return l, err
		}
	}
	l, err := c.webPlaylist(ctx, LikedSongs, likedURI, s)
	if err != nil {
		return List{}, err
	}
	l.Title, l.Owner = "Liked Songs", ""
	return l, nil
}

func (c Client) webLibrary(ctx context.Context, s Session) (List, error) {
	l := List{Ref: LikedSongs, Title: "Liked Songs"}
	for offset := 0; offset < maxSongs; offset += libraryPage {
		var data struct {
			Me *struct {
				Library struct {
					Tracks struct {
						Total int `json:"totalCount"`
						Items []struct {
							Track struct {
								URI  string   `json:"_uri"`
								Data webTrack `json:"data"`
							} `json:"track"`
						} `json:"items"`
					} `json:"tracks"`
				} `json:"library"`
			} `json:"me"`
		}
		err := c.query(ctx, s, libraryQuery, map[string]any{"offset": offset, "limit": libraryPage}, &data)
		if err != nil {
			return List{}, err
		}
		if data.Me == nil {
			return List{}, fmt.Errorf("%w: it isn't signed in", errSession)
		}
		tracks := data.Me.Library.Tracks
		for _, item := range tracks.Items {
			t := item.Track.Data
			if t.URI == "" {
				t.URI = item.Track.URI
			}
			if t.isSong() || (t.Typename == "" && strings.HasPrefix(t.URI, "spotify:track:")) {
				l.Songs = append(l.Songs, t.song())
			}
		}
		if len(tracks.Items) < libraryPage || offset+libraryPage >= tracks.Total {
			break
		}
	}
	return l, nil
}

func (c Client) webAlbum(ctx context.Context, ref Ref, s Session) (List, error) {
	var l List
	for offset := 0; offset < maxSongs; offset += webPage {
		var data struct {
			Album *struct {
				Typename string     `json:"__typename"`
				Name     string     `json:"name"`
				Artists  webArtists `json:"artists"`
				CoverArt webCover   `json:"coverArt"`
				Date     webDate    `json:"date"`
				Tracks   struct {
					Total int `json:"totalCount"`
					Items []struct {
						Track struct {
							URI      string     `json:"uri"`
							Name     string     `json:"name"`
							Artists  webArtists `json:"artists"`
							Disc     int        `json:"discNumber"`
							Number   int        `json:"trackNumber"`
							Duration struct {
								MS float64 `json:"totalMilliseconds"`
							} `json:"duration"`
						} `json:"track"`
					} `json:"items"`
				} `json:"tracksV2"`
			} `json:"albumUnion"`
		}
		err := c.query(ctx, s, albumQuery, map[string]any{
			"uri": ref.uri(), "locale": "", "offset": offset, "limit": webPage,
		}, &data)
		if err != nil {
			return List{}, err
		}
		a := data.Album
		if a == nil || (a.Typename != "" && a.Typename != "Album") {
			return List{}, ErrNotFound
		}
		if offset == 0 {
			l = List{Ref: ref, Title: strings.TrimSpace(a.Name), Owner: strings.Join(a.Artists.names(), ", "),
				Cover: largest(a.CoverArt.Sources)}
		}
		for _, item := range a.Tracks.Items {
			t := item.Track
			l.Songs = append(l.Songs, Song{
				ID: idOf(t.URI), Title: strings.TrimSpace(t.Name), Artists: t.Artists.names(),
				Album: l.Title, AlbumArtists: a.Artists.names(), Cover: l.Cover, Date: day(a.Date.ISO, a.Date.Precision),
				Disc: t.Disc, Number: t.Number, Duration: time.Duration(t.Duration.MS) * time.Millisecond,
			})
		}
		if len(a.Tracks.Items) < webPage || offset+webPage >= a.Tracks.Total {
			return l, nil
		}
	}
	return l, nil
}
