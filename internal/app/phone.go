package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"slices"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
	"rsc.io/qr"

	"github.com/Roham1299/snag/internal/applemusic"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/playlist"
	"github.com/Roham1299/snag/internal/probe"
	"github.com/Roham1299/snag/internal/remote"
	"github.com/Roham1299/snag/internal/spotify"
	"github.com/Roham1299/snag/packaging/icons"
)

// eventPhone carries a Phone when phone sending changes.
const eventPhone = "phone:changed"

func init() {
	application.RegisterEvent[Phone](eventPhone)
}

const phoneListed = 50

// phone holds the paired phones (remote) and the server for them.
type phone struct {
	remote *remote.Remote
	keeper *remote.Keeper
}

// newPhone keeps the paired phones in the file at path. Nothing listens until start.
func newPhone(a *App, path string, assets fs.FS) *phone {
	r := remote.New(phoneDownloads{a}, path)
	k := remote.NewKeeper(r, remote.Page(assets, icons.App), slog.Default())
	p := &phone{remote: r, keeper: k}
	emit := func() { a.wails.Event.Emit(eventPhone, p.status()) }
	k.OnChange(emit)
	r.OnPaired(func(d remote.Device) {
		emit()
		a.notifyPaired(d)
	})
	return p
}

// Phone is the phone-sending state, as Settings shows it.
type Phone struct {
	On bool `json:"on"`
	// Address is like "http://192.168.1.5:41234"; "" while not listening, as when the computer
	// is on a different network than Network, the one it was turned on in.
	Address string        `json:"address"`
	Network string        `json:"network"`
	Devices []PhoneDevice `json:"devices"`
}

// PhoneDevice is a paired phone.
type PhoneDevice struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Added int64  `json:"added"` // milliseconds since 1970
	Seen  int64  `json:"seen"`
}

// Pairing is the QR code for Link: Path is an SVG path on a Size × Size grid, without the
// quiet border.
type Pairing struct {
	Link  string `json:"link"`
	Path  string `json:"path"`
	Size  int    `json:"size"`
	Until int64  `json:"until"` // milliseconds since 1970
}

func (p *phone) status() Phone {
	s := Phone{On: p.keeper.On(), Devices: []PhoneDevice{}}
	if networks := p.keeper.Networks(); len(networks) > 0 {
		s.Network = networks[0]
	}
	if addrs := p.keeper.Addresses(); len(addrs) > 0 {
		s.Address = addrs[0]
	}
	for _, d := range p.remote.Devices() {
		s.Devices = append(s.Devices, PhoneDevice{
			ID: d.ID, Name: d.Name, Added: d.Added.UnixMilli(), Seen: d.Seen.UnixMilli(),
		})
	}
	return s
}

// PhoneService backs the phone settings.
type PhoneService struct {
	app *App
}

func (s *PhoneService) phone() (*phone, error) {
	if s.app.phone == nil {
		return nil, errors.New("sending from a phone isn't available in this copy of Snag")
	}
	return s.app.phone, nil
}

// Status returns the phone-sending state.
func (s *PhoneService) Status() (Phone, error) {
	p, err := s.phone()
	if err != nil {
		return Phone{}, err
	}
	return p.status(), nil
}

// Set turns phone sending on, for the current network, or off.
func (s *PhoneService) Set(on bool) (Phone, error) {
	p, err := s.phone()
	if err != nil {
		return Phone{}, err
	}
	if !on {
		p.keeper.TurnOff()
		return p.status(), nil
	}
	if err := p.keeper.TurnOn(); err != nil {
		if errors.Is(err, remote.ErrNoNetwork) {
			err = errors.New("this computer isn't on a home network Snag can listen on")
		}
		return p.status(), err
	}
	return p.status(), nil
}

// Pair starts pairing with a new code and returns its QR code.
func (s *PhoneService) Pair() (Pairing, error) {
	p, err := s.phone()
	if err != nil {
		return Pairing{}, err
	}
	addrs := p.keeper.Addresses()
	if len(addrs) == 0 {
		return Pairing{}, errors.New("not listening for phones on this network")
	}
	code, until := p.remote.StartPairing()
	// After #, so browsers never send the code over the network.
	link := addrs[0] + "/#pair=" + code
	path, size, err := qrPath(link)
	if err != nil {
		p.remote.StopPairing()
		return Pairing{}, err
	}
	return Pairing{Link: link, Path: path, Size: size, Until: until.UnixMilli()}, nil
}

// StopPairing ends pairing.
func (s *PhoneService) StopPairing() {
	if p, err := s.phone(); err == nil {
		p.remote.StopPairing()
	}
}

// Remove unpairs a phone.
func (s *PhoneService) Remove(id string) (Phone, error) {
	p, err := s.phone()
	if err != nil {
		return Phone{}, err
	}
	p.remote.RemoveDevice(id)
	return p.status(), nil
}

// qrPath draws one rectangle per run of dark squares in a row.
func qrPath(text string) (string, int, error) {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return "", 0, err
	}
	var b strings.Builder
	for y := range code.Size {
		for x := 0; x < code.Size; {
			if !code.Black(x, y) {
				x++
				continue
			}
			run := 1
			for x+run < code.Size && code.Black(x+run, y) {
				run++
			}
			fmt.Fprintf(&b, "M%d %dh%dv1h-%dz", x, y, run, run)
			x += run
		}
	}
	return b.String(), code.Size, nil
}

// notifyPaired ignores the notification setting, so the user can remove a phone that isn't theirs.
func (a *App) notifyPaired(d remote.Device) {
	err := a.notes.SendNotification(notifications.NotificationOptions{
		ID:    "snag-phone-" + d.ID,
		Title: "A phone was paired with Snag",
		Body:  d.Name + ". If it isn't yours, remove it in Settings, Phone.",
	})
	if err != nil {
		slog.Warn("couldn't show a notification", "err", err)
	}
}

type phoneDownloads struct {
	app *App
}

var (
	errOpenedList   = errors.New("that's a list: Snag opened it on the computer, to pick what to download")
	errNotDone      = errors.New("only finished downloads can be cleared from a phone")
	errNoSuchChoice = errors.New("that quality isn't offered any more: send the link again")
)

// Look says what a link from a phone offers, as the extension's card does: a video's qualities,
// a list to pick from on the computer, or nothing for a file.
func (p phoneDownloads) Look(link string) (remote.Look, error) {
	svc, err := p.app.service()
	if err != nil {
		return remote.Look{}, err
	}
	_, fromSpotify := spotify.Parse(link)
	_, fromApple := applemusic.Parse(link)
	if fromSpotify || fromApple || playlist.LooksLikeFeed(link) {
		return remote.Look{List: true}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), addTimeout)
	defer cancel()
	r := svc.Prober.Probe(ctx, probe.Request{URL: link})
	switch {
	case !r.Supported && r.Reason != "":
		return remote.Look{}, errors.New(r.Reason)
	case !r.Supported:
		return remote.Look{}, nil
	case len(r.Choices) == 0 && r.Playlist != nil:
		return remote.Look{Title: r.Playlist.Title, Site: r.Site, List: true, Count: r.Playlist.Count}, nil
	}
	look := remote.Look{Title: r.Title, Site: r.Site, Duration: r.Duration}
	for _, c := range r.Choices {
		look.Choices = append(look.Choices, remote.Option{Label: c.Label, Detail: c.Detail, Size: c.Size})
	}
	return look, nil
}

// Add adds a link from a phone straight to the download folder, at one of Look's choices or
// Snag's default (choice -1), and notifies the user.
func (p phoneDownloads) Add(link string, choice int) (remote.Item, error) {
	if choice < 0 {
		added, err := p.app.addLink(link, true)
		if err != nil {
			return remote.Item{}, err
		}
		if added.Playlist != "" {
			return remote.Item{}, errOpenedList
		}
		if !added.Duplicate {
			p.app.notifyFromPhone(added.Download)
		}
		return itemOf(added.Download), nil
	}
	svc, err := p.app.service()
	if err != nil {
		return remote.Item{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), addTimeout)
	defer cancel()
	r := svc.Prober.Probe(ctx, probe.Request{URL: link}) // cached since Look
	if !r.Supported || choice >= len(r.Choices) {
		return remote.Item{}, errNoSuchChoice
	}
	c := r.Choices[choice]
	d, duplicate, err := svc.Engine.Add(engine.AddRequest{
		URL: link, Page: link, Title: r.Title, Label: c.Label, Kind: engine.KindStream,
		Format: c.Format, Sort: c.Sort, Audio: c.Audio, Confirmed: true,
	})
	if err != nil {
		return remote.Item{}, err
	}
	if !duplicate {
		p.app.notifyFromPhone(viewOf(d))
	}
	return itemOf(viewOf(d)), nil
}

func (a *App) notifyFromPhone(d Download) {
	svc, err := a.service()
	if err != nil || !svc.Config().Notify {
		return
	}
	err = a.notes.SendNotification(notifications.NotificationOptions{
		ID:    "snag-from-phone-" + d.ID,
		Title: "Downloading from your phone",
		Body:  d.Name,
	})
	if err != nil {
		slog.Warn("couldn't show a notification", "err", err)
	}
}

// List returns the newest downloads, skipping ones being removed.
func (p phoneDownloads) List() []remote.Item {
	svc, err := p.app.service()
	if err != nil {
		return nil
	}
	list := svc.Engine.List()
	list = slices.DeleteFunc(list, func(d engine.Download) bool { return d.Status == engine.StatusRemoving })
	items := make([]remote.Item, 0, min(len(list), phoneListed))
	for _, d := range list[:min(len(list), phoneListed)] {
		items = append(items, itemOf(viewOf(d)))
	}
	return items
}

func (p phoneDownloads) Pause(id string) error {
	return p.withEngine(func(e *engine.Engine) error { return e.Pause(id) })
}

func (p phoneDownloads) Resume(id string) error {
	return p.withEngine(func(e *engine.Engine) error { return e.Resume(id) })
}

// Forget takes a finished download off the list, keeping its file. Unfinished ones are refused
// since removing them deletes their partial data.
func (p phoneDownloads) Forget(id string) error {
	return p.withEngine(func(e *engine.Engine) error {
		d, err := e.Get(id)
		if err != nil {
			return err
		}
		if d.Status != engine.StatusDone {
			return errNotDone
		}
		return e.Remove(id, false)
	})
}

func (p phoneDownloads) withEngine(f func(*engine.Engine) error) error {
	svc, err := p.app.service()
	if err != nil {
		return err
	}
	return f(svc.Engine)
}

func itemOf(d Download) remote.Item {
	return remote.Item{ID: d.ID, Name: d.Name, Status: d.Status, Done: d.Done, Size: d.Size, Speed: d.Speed}
}
