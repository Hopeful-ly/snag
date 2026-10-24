package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/ipc"
	"github.com/Roham1299/snag/internal/platform"
	"github.com/Roham1299/snag/internal/service"
	"github.com/Roham1299/snag/internal/tools"
)

type message map[string]any

func (m message) str(key string) string { s, _ := m[key].(string); return s }
func (m message) ok() bool              { ok, _ := m["ok"].(bool); return ok }
func (m message) obj(key string) message {
	o, _ := m[key].(map[string]any)
	return o
}

type testApp struct {
	*service.Service
	paths  platform.Paths
	window *window
}

func startApp(t *testing.T, settings string) *testApp {
	t.Helper()
	root := t.TempDir()
	paths := platform.Paths{
		Config:    filepath.Join(root, "settings.json"),
		State:     filepath.Join(root, "list.json"),
		Work:      filepath.Join(root, "work"),
		Downloads: filepath.Join(root, "downloads"),
		Socket:    socketAddress(t),
	}
	if settings != "" {
		if err := os.WriteFile(paths.Config, []byte(settings), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s, err := service.Start(service.Options{Paths: paths, Tools: tools.Paths{}, Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	app := &testApp{Service: s, paths: paths, window: &window{}}
	s.SetWindow(app.window)
	t.Cleanup(func() { s.Close(context.Background()) })
	return app
}

func socketAddress(t *testing.T) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(`\\.\pipe\snag-test-%d-%s`, os.Getpid(), strings.ReplaceAll(t.Name(), "/", "-"))
	}
	dir, err := os.MkdirTemp("", "snag") // socket paths are limited to ~100 bytes
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "app.sock")
}

// client talks to the app the way the browser helper does.
type client struct {
	conn    *ipc.Conn
	rid     int
	mu      sync.Mutex
	replies map[int]chan message
	events  chan message
}

func connect(t *testing.T, app *testApp) *client {
	t.Helper()
	c, err := ipc.Dial(app.paths.Socket, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	cl := &client{conn: ipc.NewConn(c), replies: map[int]chan message{}, events: make(chan message, 1000)}
	t.Cleanup(func() { cl.conn.Close() })
	go cl.read()
	return cl
}

func (c *client) read() {
	for {
		raw, err := c.conn.Receive()
		if err != nil {
			close(c.events)
			return
		}
		var m message
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		if m.str("type") != "reply" {
			c.events <- m
			continue
		}
		rid, _ := m["rid"].(float64)
		c.mu.Lock()
		ch := c.replies[int(rid)]
		c.mu.Unlock()
		if ch != nil {
			ch <- m
		}
	}
}

func (c *client) ask(t *testing.T, kind string, fields ...any) message {
	t.Helper()
	c.mu.Lock()
	c.rid++
	rid := c.rid
	ch := make(chan message, 1)
	c.replies[rid] = ch
	c.mu.Unlock()
	req := message{"type": kind, "rid": rid}
	for i := 0; i+1 < len(fields); i += 2 {
		req[fields[i].(string)] = fields[i+1]
	}
	if err := c.conn.Send(req); err != nil {
		t.Fatal(err)
	}
	select {
	case reply := <-ch:
		return reply
	case <-time.After(30 * time.Second):
		t.Fatalf("no reply to %s", kind)
		return nil
	}
}

func (c *client) waitEvent(t *testing.T, match func(message) bool) message {
	t.Helper()
	deadline := time.After(30 * time.Second)
	for {
		select {
		case m, ok := <-c.events:
			if !ok {
				t.Fatal("connection closed")
			}
			if match(m) {
				return m
			}
		case <-deadline:
			t.Fatal("timed out waiting for an event")
		}
	}
}

type window struct {
	mu    sync.Mutex
	shown []string
}

func (w *window) Show(id string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.shown = append(w.shown, id)
}

// ShowPlaylist records "playlist <id>".
func (w *window) ShowPlaylist(id string) {
	w.Show("playlist " + id)
}

// ShowLinks records "links <id>".
func (w *window) ShowLinks(id string) {
	w.Show("links " + id)
}

func (w *window) last() (string, int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.shown) == 0 {
		return "", 0
	}
	return w.shown[len(w.shown)-1], len(w.shown)
}
