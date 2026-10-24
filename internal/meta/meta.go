// Package meta describes a video or song for labelling its file where Snag knows more than its
// source, such as the song a Spotify playlist names.
package meta

// Tags are what a file is labelled with. Empty fields are unset.
type Tags struct {
	Title       string `json:"title,omitempty"`
	Artist      string `json:"artist,omitempty"` // comma-separated
	Album       string `json:"album,omitempty"`
	AlbumArtist string `json:"album_artist,omitempty"`
	Track       int    `json:"track,omitempty"`
	Disc        int    `json:"disc,omitempty"`
	Show        string `json:"show,omitempty"`
	Season      int    `json:"season,omitempty"`
	Episode     int    `json:"episode,omitempty"`
	Date        string `json:"date,omitempty"` // YYYY-MM-DD, or YYYY alone
	Page        string `json:"page,omitempty"`
	// Cover is an image URL to embed instead of the video's own thumbnail.
	Cover string `json:"cover,omitempty"`
}

// IsSet reports whether any field is set.
func (t Tags) IsSet() bool {
	return t != Tags{}
}

// Or fills t's empty fields from other.
func (t Tags) Or(other Tags) Tags {
	str := func(a *string, b string) {
		if *a == "" {
			*a = b
		}
	}
	num := func(a *int, b int) {
		if *a == 0 {
			*a = b
		}
	}
	str(&t.Title, other.Title)
	str(&t.Artist, other.Artist)
	str(&t.Album, other.Album)
	str(&t.AlbumArtist, other.AlbumArtist)
	num(&t.Track, other.Track)
	num(&t.Disc, other.Disc)
	str(&t.Show, other.Show)
	num(&t.Season, other.Season)
	num(&t.Episode, other.Episode)
	str(&t.Date, other.Date)
	str(&t.Page, other.Page)
	str(&t.Cover, other.Cover)
	return t
}

// Label is how a saved file is labelled beyond what its source says.
type Label struct {
	// Tags override the source.
	Tags Tags `json:"tags,omitzero"`
	// Defaults apply where neither Tags nor the source say anything.
	Defaults Tags `json:"defaults,omitzero"`
	// Only means the source's metadata is wrong, so the file carries only Tags.
	Only bool `json:"only,omitempty"`
}

// IsSet reports whether l labels the file at all.
func (l Label) IsSet() bool {
	return l != Label{}
}

// Known is what l says of a file before its source is read: Tags, then title, then Defaults.
func (l Label) Known(title string) Tags {
	if l.Only {
		return l.Tags
	}
	return l.Tags.Or(Tags{Title: title}).Or(l.Defaults)
}
