package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Roham1299/snag/internal/checksum"
	"github.com/Roham1299/snag/internal/cookies"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/httpdl"
)

// maxSumsSize is in bytes; maxPageFiles caps how many of the page's checksum files are tried.
const (
	maxSumsSize   = 1 << 20
	sumsTimeout   = 15 * time.Second
	maxPageFiles  = 4
	gitHubAPIBase = "https://api.github.com"
)

// softwareFile matches files that sites publish checksums for.
var softwareFile = regexp.MustCompile(`(?i)\.(iso|img|zip|7z|rar|exe|msi|msix|dmg|pkg|deb|rpm|apk|appimage|flatpak|snap|jar|bin|run|tar|tgz|tbz2?|txz|tar\.(gz|bz2|xz|zst|lz)|gz|bz2|xz|zst)$`)

// gitHubRelease matches /owner/repo/releases/download/tag/file and
// /owner/repo/releases/latest/download/file.
var gitHubRelease = regexp.MustCompile(`^/([^/]+)/([^/]+)/releases/(?:download/([^/]+)|latest/download)/([^/]+)$`)

// checksumFinder looks online for a finished download's checksum, trying the page's checksum
// files, then GitHub's release record, then checksum files beside it on its server.
type checksumFinder struct {
	client *http.Client
	// Overridden in tests.
	gitHubAPI  string
	gitHubHost string
}

func newChecksumFinder() *checksumFinder {
	return &checksumFinder{
		client:     &http.Client{Timeout: sumsTimeout},
		gitHubAPI:  gitHubAPIBase,
		gitHubHost: "github.com",
	}
}

// Find implements engine.ChecksumFinder.
func (f *checksumFinder) Find(ctx context.Context, d engine.Download) (checksum.Sum, string, error) {
	u, err := url.Parse(d.URL)
	if err != nil {
		return checksum.Sum{}, "", err
	}
	names := fileNames(d, u)
	var errs []error
	for _, file := range d.ChecksumFiles[:min(len(d.ChecksumFiles), maxPageFiles)] {
		sum, err := f.fromFile(ctx, d, file, names)
		if err != nil {
			errs = append(errs, err)
		} else if !sum.IsZero() {
			return sum, engine.FromFile, nil
		}
	}
	if m := gitHubRelease.FindStringSubmatch(u.EscapedPath()); m != nil && strings.EqualFold(u.Host, f.gitHubHost) {
		sum, err := f.fromGitHub(ctx, m[1], m[2], m[3], m[4])
		if err != nil {
			errs = append(errs, err)
		} else if !sum.IsZero() {
			return sum, engine.FromGitHub, nil
		}
	}
	if softwareFile.MatchString(names[0]) {
		for _, file := range besideIt(u) {
			sum, err := f.fromFile(ctx, d, file, names)
			if err != nil {
				errs = append(errs, err)
			} else if !sum.IsZero() {
				return sum, engine.FromFile, nil
			}
		}
	}
	if ctx.Err() != nil {
		return checksum.Sum{}, "", ctx.Err()
	}
	// Most files have no checksum anywhere, so missing ones are only an error if the page linked some.
	if len(errs) > 0 && len(d.ChecksumFiles) > 0 {
		return checksum.Sum{}, "", errors.Join(errs...)
	}
	return checksum.Sum{}, "", nil
}

// fileNames are the names a checksum file may list the download under: the URL's name first,
// then the saved name.
func fileNames(d engine.Download, u *url.URL) []string {
	var names []string
	if base := path.Base(u.Path); base != "/" && base != "." {
		if unescaped, err := url.PathUnescape(base); err == nil {
			base = unescaped
		}
		names = append(names, base)
	}
	if d.Path != "" {
		names = append(names, filepath.Base(d.Path))
	}
	if len(names) == 0 {
		names = append(names, d.Name())
	}
	return names
}

func besideIt(u *url.URL) []string {
	file := *u
	file.RawQuery, file.Fragment = "", ""
	dir := file
	dir.Path, dir.RawPath = path.Dir(u.Path)+"/", ""
	out := []string{}
	for _, ext := range []string{".sha256", ".sha256sum", ".sha512"} {
		out = append(out, file.String()+ext)
	}
	for _, name := range []string{"SHA256SUMS", "sha256sums.txt", "SHA512SUMS"} {
		out = append(out, dir.JoinPath(name).String())
	}
	return out
}

// fromFile returns a zero Sum when the checksum file is missing or doesn't list the download.
func (f *checksumFinder) fromFile(ctx context.Context, d engine.Download, address string, names []string) (checksum.Sum, error) {
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return checksum.Sum{}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return checksum.Sum{}, err
	}
	req.Header.Set("User-Agent", httpdl.UserAgent)
	if referer := d.Headers["Referer"]; referer != "" {
		req.Header.Set("Referer", referer)
	}
	if cookie := cookies.Header(d.Cookies, u); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return checksum.Sum{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return checksum.Sum{}, nil
	}
	if resp.ContentLength > maxSumsSize || strings.Contains(resp.Header.Get("Content-Type"), "html") {
		return checksum.Sum{}, nil // a page saying it isn't there, or something else entirely
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSumsSize+1))
	if err != nil || len(body) > maxSumsSize {
		return checksum.Sum{}, err
	}
	for _, name := range names {
		if sum, ok := checksum.Find(body, name); ok {
			return sum, nil
		}
	}
	return checksum.Sum{}, nil
}

// fromGitHub reads a file's SHA-256 from the release's "digest" field (GitHub added it in
// mid-2025). tag "" is the latest release.
func (f *checksumFinder) fromGitHub(ctx context.Context, owner, repo, tag, file string) (checksum.Sum, error) {
	release := "/latest"
	if tag != "" {
		release = "/tags/" + tag // still escaped, as it came in the address
	}
	address := fmt.Sprintf("%s/repos/%s/%s/releases%s", f.gitHubAPI, owner, repo, release)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return checksum.Sum{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Snag")
	resp, err := f.client.Do(req)
	if err != nil {
		return checksum.Sum{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return checksum.Sum{}, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var answer struct {
		Assets []struct {
			Name   string `json:"name"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&answer); err != nil {
		return checksum.Sum{}, err
	}
	if unescaped, err := url.PathUnescape(file); err == nil {
		file = unescaped
	}
	for _, a := range answer.Assets {
		if a.Name == file {
			if sum, ok := checksum.Parse(a.Digest); ok {
				return sum, nil
			}
		}
	}
	return checksum.Sum{}, nil
}
