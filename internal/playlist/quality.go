package playlist

import (
	"fmt"
	"net/url"
	"strings"
)

// Quality is a format rule applied to every entry of a list, such as "1080p or the best below
// it", since entries don't share format ids.
type Quality struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// yt-dlp's -f selector, -S sort order, and audio conversion format.
	Format    string `json:"-"`
	Sort      string `json:"-"`
	Audio     string `json:"-"`
	AudioOnly bool   `json:"audioOnly,omitempty"`
}

const compatible = "vcodec:h264,acodec:aac,ext:mp4:m4a"

// Qualities are the qualities offered, best first.
var Qualities = []Quality{
	{ID: "best", Label: "Best", Format: "bv*+ba/b", Sort: compatible},
	upTo(2160), upTo(1440), upTo(1080), upTo(720), upTo(480),
	{ID: "m4a", Label: "Audio M4A", Format: "ba[ext=m4a]/ba/b", Audio: "m4a", AudioOnly: true},
	{ID: "mp3", Label: "Audio MP3", Format: "ba/b", Audio: "mp3", AudioOnly: true},
}

// asPublished downloads podcast episodes as published, without format selection or conversion.
var asPublished = Quality{ID: "published", Label: "As published", Format: "b", AudioOnly: true}

func upTo(height int) Quality {
	return Quality{
		ID:     fmt.Sprint(height),
		Label:  fmt.Sprintf("%dp", height),
		Format: fmt.Sprintf("bv*[height<=%[1]d]+ba/b[height<=%[1]d]/bv*+ba/b", height),
		Sort:   fmt.Sprintf("res:%d,fps,%s", height, compatible),
	}
}

// QualitiesFor returns the qualities offered for p: audio only for ViaSearch lists.
func QualitiesFor(p Playlist) []Quality {
	if p.Podcast {
		return []Quality{asPublished, Qualities[len(Qualities)-1]} // MP3, for old players
	}
	if p.Via != ViaSearch {
		return Qualities
	}
	var audio []Quality
	for _, q := range Qualities {
		if q.AudioOnly {
			audio = append(audio, q)
		}
	}
	return audio
}

// QualityOf returns the quality with id.
func QualityOf(id string) (Quality, bool) {
	if id == asPublished.ID {
		return asPublished, true
	}
	for _, q := range Qualities {
		if q.ID == id {
			return q, true
		}
	}
	return Quality{}, false
}

var musicHosts = []string{"music.youtube.com", "soundcloud.com", "bandcamp.com", "mixcloud.com", "audiomack.com"}

// DefaultQuality is the quality a list starts with: MP3 for music, the best video otherwise.
func DefaultQuality(p Playlist) string {
	if p.Podcast {
		return asPublished.ID
	}
	if p.Via == ViaSearch {
		return "mp3"
	}
	if u, err := url.Parse(p.Page); err == nil {
		host := strings.ToLower(u.Hostname())
		for _, music := range musicHosts {
			if host == music || strings.HasSuffix(host, "."+music) {
				return "mp3"
			}
		}
	}
	return "best"
}
