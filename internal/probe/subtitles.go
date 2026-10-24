package probe

import (
	"slices"
	"strings"

	"github.com/Roham1299/snag/internal/ytdlp"
)

// Subtitle is one language a page or stream offers captions in.
type Subtitle struct {
	Code string `json:"code"` // yt-dlp's language id, such as "en" or "es"
	Name string `json:"name"`
	Auto bool   `json:"auto,omitempty"`
}

// subtitlesOf lists every uploader track, but automatic captions only for the video's language and
// English, since YouTube ships a hundred machine translations.
func subtitlesOf(info ytdlp.Info) []Subtitle {
	var out []Subtitle
	seen := map[string]bool{}
	add := func(code string, caps []ytdlp.Caption, auto bool) {
		if skipLang(code) || len(caps) == 0 {
			return
		}
		primary := primaryLang(code)
		if primary == "" || seen[primary] {
			return
		}
		seen[primary] = true
		out = append(out, Subtitle{Code: code, Name: captionName(code, caps), Auto: auto})
	}
	for _, code := range langCodes(info.Subtitles) {
		add(code, info.Subtitles[code], false)
	}
	for _, f := range formatsOf(info) {
		if !subtitleFormat(f) {
			continue
		}
		code := strings.TrimSpace(f.Language)
		if code == "" {
			code = langFromFormatID(f.FormatID)
		}
		if code == "" {
			continue
		}
		add(code, []ytdlp.Caption{{Ext: f.Ext, URL: f.URL}}, false)
	}
	for _, code := range langCodes(info.AutomaticCaptions) {
		if !wantAuto(code, info.Language, seen) {
			continue
		}
		add(code, info.AutomaticCaptions[code], true)
	}
	slices.SortFunc(out, func(a, b Subtitle) int {
		ae, be := primaryLang(a.Code) == "en", primaryLang(b.Code) == "en"
		if ae != be {
			return boolInt(be) - boolInt(ae)
		}
		if a.Auto != b.Auto {
			return boolInt(a.Auto) - boolInt(b.Auto)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

func wantAuto(code, videoLang string, seen map[string]bool) bool {
	if skipLang(code) || seen[primaryLang(code)] {
		return false
	}
	if strings.HasSuffix(strings.ToLower(code), "-orig") {
		return true
	}
	p := primaryLang(code)
	if videoLang != "" && p == primaryLang(videoLang) {
		return true
	}
	return p == "en"
}

func skipLang(code string) bool {
	c := strings.ToLower(strings.TrimSpace(code))
	return c == "" || c == "live_chat" || strings.HasPrefix(c, "live_chat")
}

func langCodes(m map[string][]ytdlp.Caption) []string {
	codes := make([]string, 0, len(m))
	for code := range m {
		codes = append(codes, code)
	}
	slices.SortFunc(codes, func(a, b string) int {
		if pa, pb := primaryLang(a), primaryLang(b); pa != pb {
			return strings.Compare(pa, pb)
		}
		ao := strings.HasSuffix(strings.ToLower(a), "-orig")
		bo := strings.HasSuffix(strings.ToLower(b), "-orig")
		if ao != bo {
			return boolInt(ao) - boolInt(bo)
		}
		return len(a) - len(b)
	})
	return codes
}

func primaryLang(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	code = strings.TrimSuffix(code, "-orig")
	base, _, _ := strings.Cut(code, "-")
	if mapped := iso639_2[base]; mapped != "" {
		return mapped
	}
	return base
}

func subtitleFormat(f ytdlp.Format) bool {
	switch strings.ToLower(f.Ext) {
	case "vtt", "srt", "ttml", "ass", "ssa":
		return true
	}
	return false
}

func langFromFormatID(id string) string {
	for _, part := range strings.Split(strings.ToLower(id), "-") {
		if languageNames[part] != "" || iso639_2[part] != "" {
			return part
		}
	}
	return ""
}

var iso639_2 = map[string]string{
	"ara": "ar", "ces": "cs", "cze": "cs", "dan": "da", "deu": "de", "ger": "de",
	"ell": "el", "gre": "el", "eng": "en", "spa": "es", "fin": "fi", "fra": "fr",
	"fre": "fr", "heb": "he", "hin": "hi", "hun": "hu", "ind": "id", "ita": "it",
	"jpn": "ja", "kor": "ko", "nld": "nl", "dut": "nl", "nor": "no", "pol": "pl",
	"por": "pt", "ron": "ro", "rum": "ro", "rus": "ru", "swe": "sv", "tha": "th",
	"tur": "tr", "ukr": "uk", "vie": "vi", "zho": "zh", "chi": "zh",
}

func captionName(code string, caps []ytdlp.Caption) string {
	for _, c := range caps {
		name := strings.TrimSpace(c.Name)
		name = strings.TrimSuffix(name, " (auto-generated)")
		name = strings.TrimSuffix(name, " (automatic captions)")
		name = strings.TrimSpace(name)
		if name != "" {
			return name
		}
	}
	if name := languageNames[primaryLang(code)]; name != "" {
		return name
	}
	return code
}

var languageNames = map[string]string{
	"ar": "العربية", "cs": "Čeština", "da": "Dansk", "de": "Deutsch", "el": "Ελληνικά",
	"en": "English", "es": "Español", "fi": "Suomi", "fr": "Français", "he": "עברית",
	"hi": "हिन्दी", "hu": "Magyar", "id": "Indonesia", "it": "Italiano", "ja": "日本語",
	"ko": "한국어", "nl": "Nederlands", "no": "Norsk", "pl": "Polski", "pt": "Português",
	"ro": "Română", "ru": "Русский", "sv": "Svenska", "th": "ไทย", "tr": "Türkçe",
	"uk": "Українська", "vi": "Tiếng Việt", "zh": "中文",
}
