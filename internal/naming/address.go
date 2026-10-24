package naming

import (
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	shortExtension = regexp.MustCompile(`^\.[A-Za-z0-9]{2,5}$`)
	genericStem    = regexp.MustCompile(`(?i)^(index|master|playlist|manifest|download|file|get|stream|video|media)([-_. ].*)?$`)
	hexStem        = regexp.MustCompile(`^[0-9a-fA-F]{6,}$`)
	letter         = regexp.MustCompile(`[A-Za-z]`)
	digit          = regexp.MustCompile(`[0-9]`)
	separator      = regexp.MustCompile(`[-_. ]`)
)

// FromAddress returns an address's last part when it looks like a real file name, not a token,
// hash or id. Generic names like "video.mp4" count only when generic is true.
func FromAddress(address string, generic bool) string {
	u, err := url.Parse(address)
	if err != nil {
		return ""
	}
	name := path.Base(strings.TrimRight(u.Path, "/"))
	stem, ext := SplitExt(name)
	if stem == "" || stem == "." || stem == "/" || !shortExtension.MatchString(ext) {
		return ""
	}
	if !generic && genericStem.MatchString(stem) {
		return ""
	}
	letters := len(letter.FindAllString(stem, -1))
	digits := len(digit.FindAllString(stem, -1))
	if letters < 3 || digits > letters {
		return ""
	}
	if hexStem.MatchString(strings.NewReplacer("-", "", "_", "").Replace(stem)) {
		return ""
	}
	if utf8.RuneCountInString(stem) > 24 && !separator.MatchString(stem) {
		return "" // one long run of characters: a token
	}
	return name
}
