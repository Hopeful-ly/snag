// Package nativemsg implements snag-host: it relays native messaging frames between the browser
// and the app, starting the app if it isn't running.
package nativemsg

import (
	"cmp"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/Roham1299/snag/internal/ipc"
)

const defaultStartTimeout = 15 * time.Second

// allowed is what the extension may ask. Settings change only in the app, never from web-facing code.
var allowed = map[string]bool{
	"ping": true, "list": true, "subscribe": true, "add": true, "have": true,
	"probe": true, "sniff": true, "playlist": true, "links": true, "kind": true, "show": true,
	"resolved": true, "resolve_failed": true,
	"pause": true, "resume": true, "retry": true, "remove": true, "open": true, "reveal": true,
	"pause_all": true, "resume_all": true, "clear_done": true,
}

// Relay connects one browser to the app.
type Relay struct {
	Socket      string
	StartApp    func() error
	FromBrowser io.Reader // standard input
	ToBrowser   io.Writer // standard output
	// StartTimeout is the wait for a started app; 0 means 15 seconds.
	StartTimeout time.Duration

	mu sync.Mutex
}

// Run relays until the browser or the app goes away.
func (r *Relay) Run() error {
	conn, err := r.connect()
	if err != nil {
		r.send(map[string]any{"type": "error", "error": "couldn't start Snag: " + err.Error()})
		return err
	}
	app := ipc.NewConn(conn)
	defer app.Close()
	r.send(map[string]any{"type": "ready"})

	appGone := make(chan struct{})
	go func() {
		defer close(appGone)
		for {
			message, err := app.Receive()
			if err != nil {
				r.send(map[string]any{"type": "daemon_gone"})
				return
			}
			r.sendRaw(message)
		}
	}()

	browserGone := make(chan error, 1)
	go func() { browserGone <- r.fromBrowser(app) }()
	select {
	case <-appGone:
		return nil // the extension reconnects, which starts a fresh helper
	case err := <-browserGone:
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
}

// fromBrowser forwards only allowed requests to the app.
func (r *Relay) fromBrowser(app *ipc.Conn) error {
	for {
		message, err := readFrame(r.FromBrowser)
		if errors.Is(err, errTooLarge) {
			continue
		}
		if err != nil {
			return err
		}
		var req struct {
			Type string          `json:"type"`
			RID  json.RawMessage `json:"rid"`
		}
		if json.Unmarshal(message, &req) != nil || !allowed[req.Type] {
			if len(req.RID) > 0 {
				r.send(map[string]any{"type": "reply", "rid": req.RID, "ok": false, "error": "not allowed"})
			}
			continue
		}
		if err := app.SendRaw(message); err != nil {
			return err
		}
	}
}

func (r *Relay) connect() (net.Conn, error) {
	if conn, err := ipc.Dial(r.Socket, time.Second); err == nil {
		return conn, nil
	}
	if err := r.StartApp(); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(cmp.Or(r.StartTimeout, defaultStartTimeout))
	for {
		conn, err := ipc.Dial(r.Socket, time.Second)
		if err == nil {
			return conn, nil
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func (r *Relay) send(message map[string]any) {
	data, err := json.Marshal(message)
	if err == nil {
		r.sendRaw(data)
	}
}

func (r *Relay) sendRaw(message []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_ = writeFrame(r.ToBrowser, message) // too large for the browser: dropped, as the browser would
}
