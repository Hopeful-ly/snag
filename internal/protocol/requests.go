package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/Roham1299/snag/internal/config"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/ipc"
	"github.com/Roham1299/snag/internal/links"
	"github.com/Roham1299/snag/internal/playlist"
	"github.com/Roham1299/snag/internal/probe"
)

const probeTimeout = 2 * time.Minute

var errNeedURL = errors.New("need an http or https address")

// result is the fields a reply adds to "type", "rid" and "ok".
type result map[string]any

func (h *Handler) handle(conn *ipc.Conn, req request) (result, error) {
	switch req.Type {
	case "ping":
		return result{"version": Version, "ytdlp": h.HasYtDLP}, nil
	case "list":
		return result{"downloads": toWireList(h.Engine.List())}, nil
	case "subscribe":
		h.mu.Lock()
		h.subscribers[conn] = true
		h.mu.Unlock()
		return result{"downloads": toWireList(h.Engine.List())}, nil
	case "add":
		return h.add(req)
	case "have":
		return h.have(req)
	case "confirm":
		return nil, h.confirm(req)
	case "probe":
		if !isWebAddress(req.URL) {
			return nil, errNeedURL
		}
		ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
		defer cancel()
		found := h.Prober.Probe(ctx, probe.Request{URL: req.URL, Headers: req.Headers, Cookies: req.Cookies, Stream: req.Stream})
		return result{"probe": found}, nil
	case "sniff":
		if !isWebAddress(req.URL) {
			return nil, errNeedURL
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		sniffed := h.Prober.Sniff(ctx, req.URL, req.Headers, req.Cookies)
		if sniffed.Kind == "" {
			return result{"kind": nil}, nil
		}
		return result{"kind": sniffed.Kind, "text": sniffed.Text}, nil
	case "kind":
		return result{"kind": h.kindOf(req.URL)}, nil
	case "playlist":
		return h.playlist(req)
	case "links":
		return h.links(req)
	case "resolved", "resolve_failed":
		return nil, h.resolved(req)
	case "pause":
		return nil, h.Engine.Pause(req.ID)
	case "resume", "retry":
		return nil, h.Engine.Resume(req.ID)
	case "remove":
		return nil, h.Engine.Remove(req.ID, req.DeleteFile)
	case "open":
		return nil, h.Files.Open(req.ID)
	case "reveal":
		return nil, h.Files.Reveal(req.ID)
	case "pause_all":
		h.Engine.PauseAll()
		return nil, nil
	case "resume_all":
		h.Engine.ResumeAll()
		return nil, nil
	case "clear_done":
		h.Engine.ClearDone()
		return nil, nil
	case "config":
		return h.config(req)
	case "show":
		if h.Window != nil {
			h.Window.Show(req.ID)
		}
		return nil, nil
	}
	return nil, fmt.Errorf("unknown request %q", req.Type)
}

func (h *Handler) links(req request) (result, error) {
	if h.Links == nil {
		return nil, errors.New("can't download links")
	}
	source := links.FromPage
	if req.Source == links.FromSelection {
		source = links.FromSelection
	}
	set, err := links.New(req.Page, req.Title, source, req.Links)
	if err != nil {
		return nil, err
	}
	set.Headers, set.Cookies = req.Headers, req.Cookies
	set = h.Links.KeepLinks(set)
	if h.Window != nil {
		h.Window.ShowLinks(set.ID)
	}
	return result{"links": linksFound{ID: set.ID, Count: len(set.Links)}}, nil
}

func (h *Handler) playlist(req request) (result, error) {
	if !isWebAddress(req.URL) {
		return nil, errNeedURL
	}
	if h.Playlists == nil {
		return nil, errors.New("can't list playlists")
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	p, err := h.Playlists.ListPlaylist(ctx, playlist.Request{
		URL: req.URL, Title: req.Title, Headers: req.Headers, Cookies: req.Cookies, TMDB: req.TMDB,
		Spotify: req.Spotify, Feeds: req.Feeds, FeedsOnly: req.FeedsOnly, Follow: req.Subscribe,
	})
	if errors.Is(err, playlist.ErrNotAList) {
		return result{"playlist": nil}, nil
	}
	if err != nil {
		return nil, err
	}
	if h.Window != nil && (req.Open == nil || *req.Open) {
		h.Window.ShowPlaylist(p.ID)
	}
	return result{"playlist": playlistToWire(p)}, nil
}

func (h *Handler) have(req request) (result, error) {
	out := make([]any, len(req.Items))
	for i, it := range req.Items {
		d, ok := h.Engine.Have(engine.HaveQuery{
			URL: it.URL, Page: req.Page, Title: req.Title, Label: it.Label,
			Filename: it.Filename, Format: it.Format, Audio: it.Audio, Clip: it.clip(),
		})
		if ok {
			out[i] = toWire(d)
		}
	}
	return result{"have": out}, nil
}

func (h *Handler) add(req request) (result, error) {
	kind, ok := kindFromWire(req.Kind)
	if !ok {
		return nil, fmt.Errorf("unknown kind %q", req.Kind)
	}
	d, duplicate, err := h.Engine.Add(engine.AddRequest{
		URL: req.URL, Page: req.Page, Title: req.Title, Label: req.Label, Kind: kind,
		Format: req.Format, Sort: req.Sort, Audio: req.Audio,
		Subs: req.Subs, Sidecar: req.Sidecar, SubAuto: req.SubAuto, SubURL: req.SubURL, SubText: req.SubText,
		Clip:    req.clip(),
		Headers: req.Headers, Cookies: req.Cookies,
		Checksum: req.Checksum, ChecksumFiles: req.ChecksumFiles,
		Directory: req.Directory, Filename: req.Filename, Size: int64(req.Size),
		Paused:    req.Start != nil && !*req.Start,
		Confirmed: req.Ask != nil && !*req.Ask,
	})
	if err != nil {
		return nil, err
	}
	return result{"download": toWire(d), "duplicate": duplicate}, nil
}

func (h *Handler) confirm(req request) error {
	err := h.Engine.Confirm(req.ID, engine.ConfirmRequest{
		Directory: req.Directory, Filename: req.Filename, Paused: req.Start != nil && !*req.Start,
	})
	if err != nil {
		return err
	}
	d, err := h.Engine.Get(req.ID)
	if err != nil {
		return err
	}
	_, err = h.Settings.Update(func(c *config.Config) { c.AddRecentDir(d.Directory) })
	return err
}

// config returns the settings, after changing the ones in req.Set.
func (h *Handler) config(req request) (result, error) {
	if len(req.Set) == 0 {
		return result{"config": settingsToWire(h.Settings.Config())}, nil
	}
	current := settingsToWire(h.Settings.Config())
	data, _ := json.Marshal(current)
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(data, &fields)
	for key, value := range req.Set {
		old, known := fields[key]
		if !known {
			return nil, fmt.Errorf("unknown setting %s", key)
		}
		if jsonType(old) != jsonType(value) {
			return nil, fmt.Errorf("bad value for %s", key)
		}
		fields[key] = value
	}
	data, _ = json.Marshal(fields)
	var changed settings
	if err := json.Unmarshal(data, &changed); err != nil {
		return nil, fmt.Errorf("bad settings: %w", err)
	}
	c, err := h.Settings.Update(func(c *config.Config) {
		c.DownloadDir, c.Connections, c.MaxActive = changed.DownloadDir, changed.Connections, changed.MaxActive
		c.Notify, c.Ask, c.RecentDirs = changed.Notify, changed.Ask, changed.RecentDirs
	})
	if err != nil {
		return nil, err
	}
	return result{"config": settingsToWire(c)}, nil
}

// jsonType names the kind of a JSON value, for checking a new setting is like the old one.
func jsonType(value json.RawMessage) string {
	var v any
	if json.Unmarshal(value, &v) != nil {
		return ""
	}
	return fmt.Sprintf("%T", v)
}

func (h *Handler) kindOf(address string) string {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	if h.Prober.IsStream(ctx, address) {
		return "ytdlp"
	}
	return "http"
}

func isWebAddress(address string) bool {
	u, err := url.Parse(address)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
