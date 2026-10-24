// Package links tidies and groups a page's (or pasted) links, keeps them while the user picks,
// and turns the picked links into downloads.
package links

import (
	"cmp"
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/Roham1299/snag/internal/cookies"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/naming"
)

// MaxLinks is the most links one set keeps.
const MaxLinks = 5000

// Group is a link's category, for picking many at once.
type Group string

// Groups of link, in display order.
const (
	Archives   Group = "archives"
	Programs   Group = "programs"
	Videos     Group = "videos"
	Audio      Group = "audio"
	Documents  Group = "documents"
	Images     Group = "images"
	VideoPages Group = "videoPages" // downloaded through yt-dlp
	Pages      Group = "pages"
	Other      Group = "other"
)

// Groups lists every group in order.
var Groups = []Group{Archives, Programs, Videos, Audio, Documents, Images, VideoPages, Pages, Other}

// Files reports whether g holds downloadable files, which start ticked.
func (g Group) Files() bool {
	switch g {
	case Archives, Programs, Videos, Audio, Documents:
		return true
	}
	return false
}

var extensions = map[Group][]string{
	Archives:  {"zip", "rar", "7z", "tar", "gz", "tgz", "bz2", "tbz", "tbz2", "xz", "txz", "zst", "lz", "lzma", "cab", "iso", "img"},
	Programs:  {"exe", "msi", "msix", "appx", "dmg", "pkg", "deb", "rpm", "apk", "xapk", "appimage", "flatpak", "snap", "jar", "bin", "run", "sh"},
	Videos:    {"mp4", "mkv", "webm", "mov", "avi", "wmv", "flv", "m4v", "3gp", "mpg", "mpeg", "ogv", "ts", "m3u8", "mpd"},
	Audio:     {"mp3", "m4a", "aac", "flac", "wav", "ogg", "oga", "opus", "wma", "alac", "aiff", "mka"},
	Documents: {"pdf", "epub", "mobi", "azw3", "djvu", "doc", "docx", "odt", "rtf", "txt", "md", "xls", "xlsx", "ods", "csv", "ppt", "pptx", "odp", "srt", "vtt", "ass", "sub", "torrent"},
	Images:    {"jpg", "jpeg", "png", "gif", "webp", "avif", "heic", "bmp", "tif", "tiff", "svg", "ico", "psd", "raw", "cr2", "nef", "dng"},
}

var pageExtensions = []string{"html", "htm", "php", "asp", "aspx", "jsp", "cgi", "shtml", "xhtml"}

// videoPage matches single-video pages on common sites; other yt-dlp sites still download as Pages.
var videoPage = regexp.MustCompile(`(?i)^https?://(?:` + strings.Join([]string{
	`(?:www\.|m\.|music\.)?youtube\.com/(?:watch\?|shorts/|live/)`,
	`youtu\.be/[\w-]{6,}`,
	`(?:www\.)?vimeo\.com/\d+`,
	`(?:www\.)?dailymotion\.com/video/`,
	`dai\.ly/`,
	`(?:www\.)?twitch\.tv/(?:videos/\d+|\w+/clip/)`,
	`clips\.twitch\.tv/`,
	`(?:www\.)?tiktok\.com/@[^/]+/video/`,
	`(?:www\.)?(?:twitter|x)\.com/\w+/status/\d+`,
	`(?:www\.)?instagram\.com/(?:p|reel|tv)/`,
	`(?:www\.)?facebook\.com/(?:watch/?\?v=|[^/]+/videos/)`,
	`(?:www\.)?reddit\.com/r/\w+/comments/`,
	`soundcloud\.com/[^/]+/[^/?#]+`,
	`(?:\w+\.)?bandcamp\.com/(?:track|album)/`,
	`(?:www\.)?bilibili\.com/video/`,
	`(?:www\.)?rumble\.com/v`,
	`(?:www\.)?odysee\.com/@`,
	`(?:www\.)?archive\.org/details/`,
}, "|") + `)`)

// Classify returns a link's group from its address and the page's name for it.
func Classify(address, name string) Group {
	if videoPage.MatchString(address) {
		return VideoPages
	}
	ext := extensionOf(name)
	if ext == "" {
		if u, err := url.Parse(address); err == nil {
			ext = extensionOf(u.Path)
		}
	}
	for _, g := range Groups {
		if slices.Contains(extensions[g], ext) {
			return g
		}
	}
	if ext == "" || slices.Contains(pageExtensions, ext) {
		return Pages
	}
	return Other
}

func extensionOf(name string) string {
	name = strings.ToLower(path.Base(strings.ReplaceAll(name, `\`, "/")))
	if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tar.xz") ||
		strings.HasSuffix(name, ".tar.bz2") || strings.HasSuffix(name, ".tar.zst") {
		return "tar"
	}
	if i := strings.LastIndexByte(name, '.'); i >= 0 && i < len(name)-1 {
		ext := name[i+1:]
		if len(ext) <= 8 && !strings.ContainsAny(ext, " ?&=") {
			return ext
		}
	}
	return ""
}

// Found is a link as the page has it. Name is the <a download> attribute.
type Found struct {
	URL  string `json:"url"`
	Text string `json:"text,omitempty"`
	Name string `json:"name,omitempty"`
}

// Link is one link of a set, tidied and grouped.
type Link struct {
	URL   string `json:"url"`
	Text  string `json:"text,omitempty"`
	Name  string `json:"name"`
	Group Group  `json:"group"`
	// Dir is its folder below the page's: "iso/24.04" for /pub/iso/24.04/a.iso found on /pub/.
	Dir string `json:"dir,omitempty"`
	// Filled once the server is asked; Server is the name it gives.
	Size   int64  `json:"size,omitempty"`
	Server string `json:"server,omitempty"`
	Looked bool   `json:"looked,omitempty"`
}

// FileName is the name its download gets.
func (l Link) FileName() string {
	return cmp.Or(l.Server, l.Name)
}

// Set is a page's or a paste's links while the user picks from them.
type Set struct {
	ID     string `json:"id"` // assigned by the Store
	Page   string `json:"page,omitempty"`
	Title  string `json:"title"`
	Source string `json:"source"`
	Links  []Link `json:"links"`
	Note   string `json:"note,omitempty"`

	// Sent with each download, as the page's site was asked.
	Headers map[string]string `json:"-"`
	Cookies []cookies.Cookie  `json:"-"`
}

// Set sources.
const (
	FromPage      = "page"
	FromSelection = "selection"
	FromPasted    = "pasted"
)

// ErrNoLinks means there was nothing to download among the links.
var ErrNoLinks = errors.New("no links to download there")

// New makes a set of the links found: resolved against page, http(s) only, deduplicated, and
// without links to the page itself.
func New(page, title, source string, found []Found) (Set, error) {
	base, _ := url.Parse(page)
	set := Set{Page: page, Title: strings.TrimSpace(title), Source: source}
	index := map[string]int{}
	for _, f := range found {
		u, err := url.Parse(strings.TrimSpace(f.URL))
		if err != nil {
			continue
		}
		if base != nil {
			u = base.ResolveReference(u)
		}
		if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			continue
		}
		u.Fragment, u.RawFragment = "", ""
		address := u.String()
		if base != nil && address == withoutFragment(base) {
			continue // "#top" and the like
		}
		text := clean(f.Text, 200)
		if i, ok := index[address]; ok {
			if set.Links[i].Text == "" {
				set.Links[i].Text = text
			}
			continue
		}
		if len(set.Links) == MaxLinks {
			set.Note = fmt.Sprintf("The page has more links than these %d.", MaxLinks)
			break
		}
		name := naming.Safe(clean(f.Name, 255), "")
		if name == "" {
			name = naming.Safe(naming.FromAddress(address, false), "")
		}
		if name == "" {
			name = naming.Safe(cmp.Or(lastPart(u), text, u.Host), "download")
		}
		group := Classify(address, f.Name)
		if group == VideoPages && text != "" {
			name = text // the address names nothing ("watch"); yt-dlp names the file
		}
		index[address] = len(set.Links)
		set.Links = append(set.Links, Link{
			URL: address, Text: text, Name: name, Group: group, Dir: dirBelow(base, u),
		})
	}
	if len(set.Links) == 0 {
		return Set{}, ErrNoLinks
	}
	return set, nil
}

func withoutFragment(u *url.URL) string {
	c := *u
	c.Fragment, c.RawFragment = "", ""
	return c.String()
}

func lastPart(u *url.URL) string {
	if base := path.Base(u.Path); base != "/" && base != "." {
		if unescaped, err := url.PathUnescape(base); err == nil {
			return unescaped
		}
		return base
	}
	return ""
}

func clean(text string, most int) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > most {
		text = strings.TrimSpace(text[:most])
	}
	return text
}

// dirBelow is u's folder below base's, as safe names; "" when u isn't under it.
func dirBelow(base, u *url.URL) string {
	if base == nil || !strings.EqualFold(base.Host, u.Host) {
		return ""
	}
	baseDir := base.Path
	if !strings.HasSuffix(baseDir, "/") {
		baseDir = path.Dir(baseDir) + "/"
	}
	dir := path.Dir(u.Path)
	if !strings.HasPrefix(dir+"/", baseDir) {
		return ""
	}
	rest := strings.Trim(strings.TrimPrefix(dir+"/", baseDir), "/")
	if rest == "" {
		return ""
	}
	var parts []string
	for part := range strings.SplitSeq(rest, "/") {
		if unescaped, err := url.PathUnescape(part); err == nil {
			part = unescaped
		}
		if part == "." || part == ".." {
			return ""
		}
		parts = append(parts, naming.Safe(part, "folder"))
	}
	return strings.Join(parts, "/")
}

var addressIn = regexp.MustCompile(`https?://[^\s<>"'` + "`" + `]+`)

// InText finds every web address in text.
func InText(text string) []Found {
	var found []Found
	for _, m := range addressIn.FindAllString(text, -1) {
		m = strings.TrimRight(m, ".,;:!?)]}")
		found = append(found, Found{URL: m})
	}
	return found
}

// Choice is what to download of a set, and where.
type Choice struct {
	Entries   []int  `json:"entries"` // indexes into Links, in download order
	Directory string `json:"directory"`
	Queue     string `json:"queue"` // "" for Main
	// KeepFolders puts each link in its Link.Dir.
	KeepFolders bool `json:"keepFolders"`
}

// Best quality in formats every player opens, matching the context menu's video download.
const (
	bestFormat     = "bv*+ba/b"
	compatibleSort = "vcodec:h264,acodec:aac,ext:mp4:m4a"
)

// Plan returns the downloads for the links chosen, pre-confirmed since the user chose the folder.
func Plan(s Set, c Choice) ([]engine.AddRequest, error) {
	if len(c.Entries) == 0 {
		return nil, errors.New("nothing chosen")
	}
	dir := strings.TrimSpace(c.Directory)
	if dir == "" {
		return nil, errors.New("choose a folder")
	}
	var reqs []engine.AddRequest
	seen := map[int]bool{}
	for _, n := range c.Entries {
		if n < 0 || n >= len(s.Links) {
			return nil, fmt.Errorf("no link %d", n)
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		l := s.Links[n]
		req := engine.AddRequest{
			URL:       l.URL,
			Page:      s.Page,
			Headers:   s.Headers,
			Cookies:   s.Cookies,
			Directory: dir,
			Queue:     c.Queue,
			Kind:      engine.KindHTTP,
			Size:      l.Size,
			Confirmed: true,
		}
		if c.KeepFolders && l.Dir != "" {
			req.Directory = filepath.Join(dir, filepath.FromSlash(l.Dir))
		}
		if l.Group == VideoPages {
			req.Kind, req.Format, req.Sort, req.Page, req.Size = engine.KindStream, bestFormat, compatibleSort, l.URL, 0
		} else {
			req.Filename = l.FileName()
		}
		reqs = append(reqs, req)
	}
	return reqs, nil
}
