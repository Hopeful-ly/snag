// Package remote lets a paired phone add links and list, pause, resume and remove downloads.
// Nothing more: it can't read files, change settings, or delete anything from disk. A phone pairs
// with a one-time code and gets a device token. Nothing carries the messages yet: they are for
// Snag Cloud's relay. A paired phone can request an invite code, since
// an iPhone home-screen copy of the page has storage separate from Safari's and must pair itself.
package remote

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Roham1299/snag/internal/store"
)

// Downloads is what a phone may do with Snag's downloads.
type Downloads interface {
	// Look says what a link offers: its title and the qualities to choose from.
	Look(link string) (Look, error)
	// Add downloads straight to the download folder: no Save window waits on the computer.
	// choice is an index into Look's Choices, or -1 for Snag's default.
	Add(link string, choice int) (Item, error)
	List() []Item
	Pause(id string) error
	Resume(id string) error
	// Forget removes a download from the list, leaving its file.
	Forget(id string) error
}

// Item is a download as the phone sees it; it deliberately has no paths.
type Item struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Done   int64  `json:"done"`
	Size   int64  `json:"size"` // 0 while unknown
	Speed  int64  `json:"speed"`
}

// Request is one message from a phone.
type Request struct {
	Op     string `json:"op"`               // "look", "add", "list", "pause", "resume", "forget" or "invite"
	Link   string `json:"link,omitempty"`   // for look and add
	Choice *int   `json:"choice,omitempty"` // for add: an index into Look's Choices; nil for the default
	ID     string `json:"id,omitempty"`     // for pause, resume and forget
}

// Look is what a link offers. No Choices means a file, added as it is; List means a playlist,
// a channel or an album, which Snag opens on the computer to pick from.
type Look struct {
	Title    string   `json:"title,omitempty"`
	Site     string   `json:"site,omitempty"`
	Duration float64  `json:"duration,omitempty"` // seconds
	Choices  []Option `json:"choices,omitempty"`
	List     bool     `json:"list,omitempty"`
	Count    int      `json:"count,omitempty"` // the list's entries, when known
}

// Option is one quality to download a link at.
type Option struct {
	Label  string `json:"label"`  // "1080p", "Audio MP3"
	Detail string `json:"detail"` // "MP4 · HDR"
	Size   int64  `json:"size"`   // estimated; 0 when unknown
}

// Response is Snag's answer.
type Response struct {
	Items []Item `json:"items,omitempty"` // for list
	Item  *Item  `json:"item,omitempty"`  // for add
	Look  *Look  `json:"look,omitempty"`  // for look
	Code  string `json:"code,omitempty"`  // for invite
	Error string `json:"error,omitempty"`
}

// Device is a paired phone, as Settings lists it.
type Device struct {
	ID    string    `json:"id"`
	Name  string    `json:"name"` // as the phone's browser reports it
	Added time.Time `json:"added"`
	Seen  time.Time `json:"seen"`
}

// A pairing code lasts pairFor; pairTries wrong tries close pairing.
const (
	pairFor      = 10 * time.Minute
	pairTries    = 5
	maxNameRunes = 60
)

var (
	// ErrNotPairing means pairing isn't open: never opened, expired, used, or closed by wrong tries.
	ErrNotPairing = errors.New("pairing isn't open: show the QR code in Snag again")
	// ErrWrongCode is returned for a code that doesn't match.
	ErrWrongCode = errors.New("that code doesn't match")
	// ErrUnknownDevice is returned for a token no paired device holds.
	ErrUnknownDevice = errors.New("this phone isn't paired with Snag")
)

// saved keeps only hashes of device tokens.
type saved struct {
	Devices []device `json:"devices,omitempty"`
}

type device struct {
	Device
	Hash string `json:"hash"` // SHA-256 of the token, in hex
}

// Remote holds the paired devices and pairing state, and answers requests.
type Remote struct {
	downloads Downloads
	file      *store.File[saved]
	now       func() time.Time

	mu          sync.Mutex
	saved       saved
	code        string // the open pairing code, from Settings; "" when pairing is closed
	until       time.Time
	invite      string // a code a paired phone asked for, kept apart so it doesn't close the QR code
	inviteUntil time.Time
	wrong       int
	onPair      func(Device)
}

// New returns a Remote for downloads, keeping its devices in the file at path.
func New(downloads Downloads, path string) *Remote {
	r := &Remote{downloads: downloads, file: store.NewFile[saved](path), now: time.Now}
	if err := r.file.Load(&r.saved); err != nil {
		slog.Warn("couldn't read the paired phones", "err", err)
	}
	return r
}

// OnPaired calls f when a phone pairs.
func (r *Remote) OnPaired(f func(Device)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onPair = f
}

// StartPairing opens pairing with a new one-time code, replacing any earlier one, and returns
// it and when it expires.
func (r *Remote) StartPairing() (code string, until time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.code = randomString(16)
	r.until = r.now().Add(pairFor)
	r.wrong = 0
	return r.code, r.until
}

// StopPairing closes pairing.
func (r *Remote) StopPairing() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.code = ""
}

// Invite returns a one-time code for a paired phone to pair another copy of the page; it doesn't touch the Settings code.
func (r *Remote) Invite() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.invite = randomString(16)
	r.inviteUntil = r.now().Add(pairFor)
	r.wrong = 0
	return r.invite
}

// Pair trades a one-time code (from Settings or an invite) for a device token. Only the token's hash is kept.
func (r *Remote) Pair(code, name string) (token string, d Device, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	if now.After(r.until) {
		r.code = ""
	}
	if now.After(r.inviteUntil) {
		r.invite = ""
	}
	if r.code == "" && r.invite == "" {
		return "", Device{}, ErrNotPairing
	}
	switch {
	case matches(code, r.code):
		r.code = ""
	case matches(code, r.invite):
		r.invite = ""
	default:
		r.wrong++
		if r.wrong >= pairTries {
			r.code, r.invite = "", ""
		}
		return "", Device{}, ErrWrongCode
	}
	token = randomString(32)
	d = Device{ID: randomString(9), Name: cleanName(name), Added: now, Seen: now}
	r.saved.Devices = append(r.saved.Devices, device{Device: d, Hash: hashOf(token)})
	r.save()
	if r.onPair != nil {
		go r.onPair(d)
	}
	return token, d, nil
}

// Devices returns the paired devices, oldest first.
func (r *Remote) Devices() []Device {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Device, len(r.saved.Devices))
	for i, d := range r.saved.Devices {
		out[i] = d.Device
	}
	return out
}

// RemoveDevice unpairs a device: its token stops working at once.
func (r *Remote) RemoveDevice(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.saved.Devices = slices.DeleteFunc(r.saved.Devices, func(d device) bool { return d.ID == id })
	r.save()
}

// Device returns the device holding token and updates its last-seen time.
func (r *Remote) Device(token string) (Device, error) {
	hash := []byte(hashOf(token))
	r.mu.Lock()
	defer r.mu.Unlock()
	found := -1
	for i, d := range r.saved.Devices { // every one compared, in constant time
		if subtle.ConstantTimeCompare(hash, []byte(d.Hash)) == 1 {
			found = i
		}
	}
	if found < 0 {
		return Device{}, ErrUnknownDevice
	}
	d := &r.saved.Devices[found]
	if now := r.now(); now.Sub(d.Seen) > time.Minute { // not a write for every request
		d.Seen = now
		r.save()
	}
	return d.Device, nil
}

// Handle answers a paired device's request.
func (r *Remote) Handle(req Request) Response {
	fail := func(err error) Response { return Response{Error: err.Error()} }
	switch req.Op {
	case "look", "add":
		u, err := url.Parse(req.Link)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fail(errors.New("send a link starting with http:// or https://"))
		}
		if req.Op == "look" {
			look, err := r.downloads.Look(u.String())
			if err != nil {
				return fail(err)
			}
			return Response{Look: &look}
		}
		choice := -1
		if req.Choice != nil {
			if *req.Choice < 0 {
				return fail(errors.New("which quality?"))
			}
			choice = *req.Choice
		}
		item, err := r.downloads.Add(u.String(), choice)
		if err != nil {
			return fail(err)
		}
		return Response{Item: &item}
	case "list":
		return Response{Items: r.downloads.List()}
	case "invite":
		return Response{Code: r.Invite()}
	case "pause", "resume", "forget":
		if req.ID == "" {
			return fail(errors.New("which download?"))
		}
		act := map[string]func(string) error{
			"pause": r.downloads.Pause, "resume": r.downloads.Resume, "forget": r.downloads.Forget,
		}[req.Op]
		if err := act(req.ID); err != nil {
			return fail(err)
		}
		return Response{}
	default:
		return fail(fmt.Errorf("unknown request %q", req.Op))
	}
}

// save writes the file; r.mu is held.
func (r *Remote) save() {
	if err := r.file.Save(r.saved); err != nil {
		slog.Warn("couldn't keep the paired phones", "err", err)
	}
}

// matches reports whether code is the open code want, in constant time.
func matches(code, want string) bool {
	return want != "" && subtle.ConstantTimeCompare([]byte(code), []byte(want)) == 1
}

func hashOf(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand doesn't fail on supported systems
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// cleanName keeps a device's name short and printable.
func cleanName(name string) string {
	name = strings.Join(strings.FieldsFunc(name, func(r rune) bool {
		return r < ' ' || r == 0x7f || r == ' '
	}), " ")
	if runes := []rune(name); len(runes) > maxNameRunes {
		name = string(runes[:maxNameRunes])
	}
	if name == "" {
		return "A phone"
	}
	return name
}
