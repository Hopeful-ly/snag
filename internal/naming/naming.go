// Package naming decides file names for downloads: it cleans titles into names every OS accepts,
// reads the name a server suggests, and tells real file names in addresses from tokens.
package naming

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Most file systems allow 255 bytes; this leaves room for " (12)" and a part-file suffix.
const maxNameBytes = 200

var (
	unsafeChars = regexp.MustCompile(`[\x00-\x1f/\\:*?"<>|]+`)
	spaces      = regexp.MustCompile(`\s+`)
	// Names Windows reserves for devices, with or without an extension.
	reservedStem = regexp.MustCompile(`(?i)^(con|prn|aux|nul|com[1-9]|lpt[1-9])$`)
)

// Safe turns name into a file name every OS accepts, or returns fallback if nothing is left of it.
func Safe(name, fallback string) string {
	name = unsafeChars.ReplaceAllString(name, " ")
	name = strings.Trim(name, " .")
	name = spaces.ReplaceAllString(name, " ")
	if len(name) > maxNameBytes {
		stem, ext := SplitExt(name)
		stem = truncateUTF8(stem, maxNameBytes-len(ext))
		name = strings.TrimRight(stem, " ") + ext
	}
	if name == "" {
		return fallback
	}
	if stem, ext := SplitExt(name); reservedStem.MatchString(stem) {
		name = stem + "_" + ext
	}
	return name
}

// SplitExt splits name into stem and extension. A leading dot doesn't start one: ".profile" has none.
func SplitExt(name string) (stem, ext string) {
	i := strings.LastIndexByte(name, '.')
	if i <= 0 {
		return name, ""
	}
	return name[:i], name[i:]
}

// FromContentDisposition returns the file name a Content-Disposition header suggests, or "".
func FromContentDisposition(header string) string {
	if header == "" {
		return ""
	}
	if _, params, err := mime.ParseMediaType(header); err == nil {
		return params["filename"] // also decodes filename*=UTF-8''...
	}
	// Servers often send names that aren't valid parameters, such as unquoted ones with spaces.
	if m := looseFilename.FindStringSubmatch(header); m != nil {
		return strings.Trim(strings.TrimSpace(m[1]), `"`)
	}
	return ""
}

var looseFilename = regexp.MustCompile(`(?i)filename\s*=\s*("[^"]+"|[^;]+)`)

var mimeExtensions = map[string]string{
	"video/mp4":                    ".mp4",
	"video/webm":                   ".webm",
	"video/x-matroska":             ".mkv",
	"video/quicktime":              ".mov",
	"video/mp2t":                   ".ts",
	"audio/mpeg":                   ".mp3",
	"audio/mp4":                    ".m4a",
	"audio/ogg":                    ".ogg",
	"audio/flac":                   ".flac",
	"audio/wav":                    ".wav",
	"image/jpeg":                   ".jpg",
	"image/png":                    ".png",
	"image/gif":                    ".gif",
	"image/webp":                   ".webp",
	"application/pdf":              ".pdf",
	"application/zip":              ".zip",
	"application/gzip":             ".gz",
	"application/x-tar":            ".tar",
	"application/x-7z-compressed":  ".7z",
	"application/vnd.rar":          ".rar",
	"application/x-rar-compressed": ".rar",
	"application/x-iso9660-image":  ".iso",
	"application/json":             ".json",
	"text/plain":                   ".txt",
}

// WithExtension adds the extension that matches the content type when name has none.
// It uses a fixed table rather than the OS's, so a download is named the same everywhere.
func WithExtension(name, contentType string) string {
	if _, ext := SplitExt(name); ext != "" && len(ext) <= 6 {
		return name
	}
	mediaType, _, _ := strings.Cut(contentType, ";")
	return name + mimeExtensions[strings.ToLower(strings.TrimSpace(mediaType))]
}

// UniquePath returns a path in dir for name that no file has and inUse doesn't claim, adding
// " (1)", " (2)"... before the extension as needed. inUse may be nil.
func UniquePath(dir, name string, inUse func(path string) bool) string {
	stem, ext := SplitExt(name)
	for n := 0; ; n++ {
		candidate := name
		if n > 0 {
			candidate = fmt.Sprintf("%s (%d)%s", stem, n, ext)
		}
		path := filepath.Join(dir, candidate)
		if _, err := os.Lstat(path); err == nil {
			continue
		}
		if inUse != nil && inUse(path) {
			continue
		}
		return path
	}
}

func truncateUTF8(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	s = s[:maxBytes]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
