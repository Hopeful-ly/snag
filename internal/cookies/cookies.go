// Package cookies sends browser cookies with downloads, as a Cookie header or a yt-dlp cookie file.
package cookies

import (
	"cmp"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Cookie is a browser cookie as the extension reports it.
type Cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path,omitempty"`
	Secure   bool    `json:"secure,omitempty"`
	HostOnly bool    `json:"hostOnly,omitempty"`
	Expires  float64 `json:"expirationDate,omitempty"` // seconds since 1970; 0 for session cookies
}

func (c Cookie) matches(host string) bool {
	domain := strings.TrimPrefix(c.Domain, ".")
	return host == domain || (!c.HostOnly && strings.HasSuffix(host, "."+domain))
}

// Header returns the Cookie header for u's host.
func Header(list []Cookie, u *url.URL) string {
	host := u.Hostname()
	var pairs []string
	for _, c := range list {
		if c.matches(host) {
			pairs = append(pairs, c.Name+"="+c.Value)
		}
	}
	return strings.Join(pairs, "; ")
}

// WriteFile writes the cookies to path in Netscape format, readable only by the current user.
func WriteFile(path string, list []Cookie) error {
	var b strings.Builder
	b.WriteString("# Netscape HTTP Cookie File\n")
	for _, c := range list {
		domain := c.Domain
		if !c.HostOnly && domain != "" && !strings.HasPrefix(domain, ".") {
			domain = "." + domain
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			domain, upper(!c.HostOnly), cmp.Or(c.Path, "/"), upper(c.Secure), int64(c.Expires), c.Name, c.Value)
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

// TempFile writes the cookies to a fresh temp file for one yt-dlp run. With no cookies the path is "".
func TempFile(list []Cookie) (path string, remove func(), err error) {
	if len(list) == 0 {
		return "", func() {}, nil
	}
	dir, err := os.MkdirTemp("", "snag-cookies-")
	if err != nil {
		return "", nil, err
	}
	remove = func() { _ = os.RemoveAll(dir) }
	path = filepath.Join(dir, "cookies.txt")
	if err := WriteFile(path, list); err != nil {
		remove()
		return "", nil, err
	}
	return path, remove, nil
}

func upper(b bool) string {
	if b {
		return "TRUE"
	}
	return "FALSE"
}
