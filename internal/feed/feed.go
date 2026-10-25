// Package feed reads Snag's signed update feed. The signature is checked against a key built
// into the app, so whoever serves the file can't change what Snag trusts.
//
// Wire format: {"feed": base64(payload), "signature": base64(Ed25519(payload))}, payload a Feed in JSON.
package feed

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Roham1299/snag/internal/store"
)

// URL is the feed's address, set at build time with -ldflags -X; empty disables checking.
var URL = ""

// PublicKey is the feed's Ed25519 public key, in base64.
const PublicKey = "TlC1jI5x9HLC51LwRGp+G9B+x37kC4NJWfdPqnPTjfU="

// Feed is the signed payload.
type Feed struct {
	// Issued is when the feed was signed. Older feeds are refused, so a stale one can't be replayed.
	Issued time.Time `json:"issued"`
	Snag   *Release  `json:"snag,omitempty"`
	// YtDLP is a yt-dlp --update-to target; empty means the latest release.
	YtDLP string `json:"ytdlp,omitempty"`
}

// Release is a version of Snag and its download page.
type Release struct {
	Version string `json:"version"`
	Page    string `json:"page"`
	// Files are the release's installers by kind (see FileKind), for Snag to update itself.
	Files map[string]File `json:"files,omitempty"`
}

// File is an installer and its SHA-256, which the feed's signature vouches for.
type File struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

var fileKinds = []struct {
	kind string
	name *regexp.Regexp
}{
	{"windows-amd64", regexp.MustCompile(`-windows-amd64-setup\.exe$`)},
	{"windows-arm64", regexp.MustCompile(`-windows-arm64-setup\.exe$`)},
	{"macos", regexp.MustCompile(`-macos\.dmg$`)},
	{"deb-amd64", regexp.MustCompile(`_amd64\.deb$`)},
	{"deb-arm64", regexp.MustCompile(`_arm64\.deb$`)},
	{"rpm-amd64", regexp.MustCompile(`\.x86_64\.rpm$`)},
	{"rpm-arm64", regexp.MustCompile(`\.aarch64\.rpm$`)},
	{"arch-amd64", regexp.MustCompile(`-x86_64\.pkg\.tar\.zst$`)},
	{"arch-arm64", regexp.MustCompile(`-aarch64\.pkg\.tar\.zst$`)},
}

// FileKind names the kind of installer a release file is, from its name: "windows-amd64",
// "macos", "deb-arm64" and so on, or "" for other files.
func FileKind(name string) string {
	for _, k := range fileKinds {
		if k.name.MatchString(name) {
			return k.kind
		}
	}
	return ""
}

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

type envelope struct {
	Feed      string `json:"feed"`
	Signature string `json:"signature"`
}

// A real feed is a few hundred bytes.
const maxSize = 64 << 10

var ytdlpVersion = regexp.MustCompile(`^([a-z]+@)?\d{4}\.\d{2}\.\d{2}(\.\d+)?$`)

// Open checks an envelope's signature with key and returns the feed in it.
func Open(data []byte, key ed25519.PublicKey) (Feed, error) {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return Feed{}, fmt.Errorf("not a feed: %w", err)
	}
	payload, err := base64.StdEncoding.DecodeString(env.Feed)
	if err != nil {
		return Feed{}, fmt.Errorf("not a feed: %w", err)
	}
	sig, err := base64.StdEncoding.DecodeString(env.Signature)
	if err != nil {
		return Feed{}, fmt.Errorf("not a feed: %w", err)
	}
	if len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, payload, sig) {
		return Feed{}, errors.New("the feed's signature doesn't match")
	}
	var f Feed
	if err := json.Unmarshal(payload, &f); err != nil {
		return Feed{}, fmt.Errorf("the feed can't be read: %w", err)
	}
	if f.Issued.IsZero() {
		return Feed{}, errors.New("the feed says nothing of when it was made")
	}
	if f.YtDLP != "" && !ytdlpVersion.MatchString(f.YtDLP) {
		return Feed{}, fmt.Errorf("the feed's yt-dlp version %q isn't one", f.YtDLP)
	}
	if f.Snag != nil {
		if _, ok := parseVersion(f.Snag.Version); !ok {
			return Feed{}, fmt.Errorf("the feed's Snag version %q isn't one", f.Snag.Version)
		}
		if !isHTTPS(f.Snag.Page) {
			return Feed{}, fmt.Errorf("the feed's release page %q isn't an https address", f.Snag.Page)
		}
		for kind, file := range f.Snag.Files {
			if !isHTTPS(file.URL) || !sha256Hex.MatchString(file.SHA256) {
				return Feed{}, fmt.Errorf("the feed's %s installer isn't an https address and a SHA-256", kind)
			}
		}
	}
	return f, nil
}

func isHTTPS(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.Host != ""
}

// Seal signs a feed with key and returns its envelope.
func Seal(f Feed, key ed25519.PrivateKey) ([]byte, error) {
	payload, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(envelope{
		Feed:      base64.StdEncoding.EncodeToString(payload),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload)),
	}, "", "  ")
}

// Newer returns the feed's release if it is newer than running, otherwise nil.
func (f Feed) Newer(running string) *Release {
	if f.Snag == nil {
		return nil
	}
	latest, _ := parseVersion(f.Snag.Version)
	current, ok := parseVersion(running)
	if !ok || compare(latest, current) <= 0 {
		return nil
	}
	return f.Snag
}

// parseVersion reads "1.2.3", allowing a leading "v".
func parseVersion(s string) ([3]int, bool) {
	var v [3]int
	parts := strings.Split(strings.TrimPrefix(s, "v"), ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

func compare(a, b [3]int) int {
	for i := range a {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// Checker fetches the feed and keeps the newest good one on disk, for offline starts.
type Checker struct {
	url    string
	key    ed25519.PublicKey
	file   *store.File[state]
	client *http.Client

	mu    sync.Mutex
	state state
}

// state keeps the envelope as served, so its signature is checked again when read.
type state struct {
	Envelope json.RawMessage `json:"envelope,omitempty"`
	Told     string          `json:"told,omitempty"`
}

// NewChecker returns a Checker for url, verified with key (base64) and persisted at path. An empty url never fetches.
func NewChecker(url, key, path string) (*Checker, error) {
	raw, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("the feed's key isn't an Ed25519 public key")
	}
	c := &Checker{url: url, key: raw, file: store.NewFile[state](path), client: &http.Client{Timeout: time.Minute}}
	if err := c.file.Load(&c.state); err != nil {
		slog.Warn("couldn't read what Snag knew of its feed", "err", err)
	}
	return c, nil
}

// Known returns the last good feed, or a zero Feed if there is none.
func (c *Checker) Known() Feed {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.known()
}

func (c *Checker) known() Feed {
	if len(c.state.Envelope) == 0 {
		return Feed{}
	}
	f, err := Open(c.state.Envelope, c.key)
	if err != nil {
		return Feed{}
	}
	return f
}

// Check fetches the feed, falling back to the last good one if it's unreachable, badly signed or older.
func (c *Checker) Check(ctx context.Context) Feed {
	if c.url == "" {
		return c.Known()
	}
	data, err := c.fetch(ctx)
	if err != nil {
		slog.Warn("couldn't check Snag's feed", "err", err)
		return c.Known()
	}
	fresh, err := Open(data, c.key)
	if err != nil {
		slog.Warn("Snag's feed was refused", "err", err)
		return c.Known()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if known := c.known(); fresh.Issued.Before(known.Issued) {
		slog.Warn("Snag's feed is older than the one it has", "issued", fresh.Issued, "has", known.Issued)
		return known
	}
	c.state.Envelope = data
	c.save()
	return fresh
}

// Tell reports whether to tell the user about r; true once per version.
func (c *Checker) Tell(r *Release) bool {
	if r == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state.Told == r.Version {
		return false
	}
	c.state.Told = r.Version
	c.save()
	return true
}

func (c *Checker) fetch(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", c.url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSize {
		return nil, errors.New("the feed is too large")
	}
	return data, nil
}

func (c *Checker) save() {
	if err := c.file.Save(c.state); err != nil {
		slog.Warn("couldn't keep what Snag learned from its feed", "err", err)
	}
}
