// Package protocol answers the browser extension's requests (see PROTOCOL.md) with calls on
// the engine and the prober.
package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"sync"

	"github.com/Roham1299/snag/internal/config"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/ipc"
	"github.com/Roham1299/snag/internal/links"
	"github.com/Roham1299/snag/internal/playlist"
	"github.com/Roham1299/snag/internal/probe"
)

// Version is the protocol's version, which ping reports.
const Version = 1

// Settings keeps the user's settings for the app.
type Settings interface {
	Config() config.Config
	// Update changes, saves and applies the settings; the keeper reports it via ConfigChanged.
	Update(change func(*config.Config)) (config.Config, error)
}

// Files opens downloaded files.
type Files interface {
	Open(id string) error
	// Reveal shows a download's file, or its folder, in the file manager.
	Reveal(id string) error
}

// Window is the app's window.
type Window interface {
	// Show brings the window up; with a download's id, at its Save dialog.
	Show(downloadID string)
	ShowPlaylist(id string)
	ShowLinks(id string)
}

// Links keeps sets of links for the window to choose from.
type Links interface {
	KeepLinks(set links.Set) links.Set
}

// Playlists finds the lists pages are in, and keeps them for the window to choose from.
type Playlists interface {
	// ListPlaylist returns playlist.ErrNotAList for a page of one video.
	ListPlaylist(ctx context.Context, req playlist.Request) (playlist.Playlist, error)
}

// Options configure a Handler.
type Options struct {
	Engine    *engine.Engine
	Prober    *probe.Prober
	Playlists Playlists
	Links     Links
	Settings  Settings
	Files     Files
	Window    Window
	HasYtDLP  bool
	// Contact, if set, is called on every request from the extension.
	Contact func()
}

// Handler answers requests on any number of connections.
type Handler struct {
	Options

	mu          sync.Mutex
	subscribers map[*ipc.Conn]bool
	recent      *ipc.Conn                  // the subscriber that asked something last
	resolving   map[string]*pendingResolve // by token
	stop        func()
}

// New returns a Handler that follows the engine's changes until Close.
func New(opts Options) *Handler {
	h := &Handler{Options: opts, subscribers: map[*ipc.Conn]bool{}, resolving: map[string]*pendingResolve{}}
	events, stop := opts.Engine.Subscribe()
	h.stop = stop
	go h.forward(events)
	return h
}

// Close stops following the engine.
func (h *Handler) Close() {
	h.stop()
}

// ServeListener answers every connection l accepts, until l is closed.
func (h *Handler) ServeListener(l net.Listener) error {
	for {
		c, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go h.Serve(ipc.NewConn(c))
	}
}

// Serve answers requests on one connection until it closes.
func (h *Handler) Serve(conn *ipc.Conn) {
	defer func() {
		h.mu.Lock()
		delete(h.subscribers, conn)
		if h.recent == conn {
			h.recent = nil
		}
		h.mu.Unlock()
		h.browserGone(conn)
		conn.Close()
	}()
	for {
		message, err := conn.Receive()
		if err != nil {
			return
		}
		var req request
		if err := json.Unmarshal(message, &req); err != nil {
			// Answer with the request's rid if it has one, so the sender isn't left waiting.
			var envelope struct {
				RID json.RawMessage `json:"rid"`
			}
			_ = json.Unmarshal(message, &envelope)
			_ = conn.Send(map[string]any{"type": "reply", "rid": envelope.RID, "ok": false, "error": "bad request: " + err.Error()})
			continue
		}
		if req.Type == "probe" || req.Type == "sniff" || req.Type == "kind" || req.Type == "playlist" || req.Type == "links" {
			go h.answer(conn, req) // slow; don't block the connection
		} else {
			h.answer(conn, req)
		}
	}
}

func (h *Handler) answer(conn *ipc.Conn, req request) {
	if h.Contact != nil {
		h.Contact()
	}
	reply := map[string]any{"type": "reply", "rid": req.RID}
	h.mu.Lock()
	if h.subscribers[conn] {
		h.recent = conn
	}
	h.mu.Unlock()
	result, err := h.handle(conn, req)
	if err != nil {
		reply["ok"], reply["error"] = false, err.Error()
	} else {
		reply["ok"] = true
		for k, v := range result {
			reply[k] = v
		}
	}
	if err := conn.Send(reply); err != nil {
		slog.Debug("couldn't answer a request", "type", req.Type, "err", err)
	}
}

// forward passes the engine's changes on to every subscribed connection.
func (h *Handler) forward(events <-chan engine.Event) {
	for event := range events {
		var message map[string]any
		switch event.Kind {
		case engine.EventAdded:
			message = map[string]any{"type": "added", "download": toWire(event.Download)}
		case engine.EventUpdated:
			message = map[string]any{"type": "update", "download": toWire(event.Download)}
		case engine.EventRemoved:
			message = map[string]any{"type": "removed", "id": event.Download.ID}
		case engine.EventReset:
			continue // the window hears the whole list; a download shows here when it starts
		}
		h.broadcast(message)
	}
}

// ConfigChanged tells subscribers about new settings, whoever changed them.
func (h *Handler) ConfigChanged(c config.Config) {
	h.broadcast(map[string]any{"type": "config", "config": settingsToWire(c)})
}

func (h *Handler) broadcast(message map[string]any) {
	data, err := json.Marshal(message)
	if err != nil {
		return
	}
	h.mu.Lock()
	conns := make([]*ipc.Conn, 0, len(h.subscribers))
	for c := range h.subscribers {
		conns = append(conns, c)
	}
	h.mu.Unlock()
	for _, c := range conns {
		_ = c.SendRaw(data)
	}
}
