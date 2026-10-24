package nativemsg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/ipc"
)

func TestFrames(t *testing.T) {
	var buf bytes.Buffer
	if err := writeFrame(&buf, []byte(`{"type":"ping"}`)); err != nil {
		t.Fatal(err)
	}
	got, err := readFrame(&buf)
	if err != nil || string(got) != `{"type":"ping"}` {
		t.Errorf("round trip = %q, %v", got, err)
	}
	if err := writeFrame(&buf, make([]byte, maxToBrowser+1)); !errors.Is(err, errTooLarge) {
		t.Errorf("an oversized frame for the browser: %v", err)
	}
}

// The extension's requests (its Request type and the actions on downloads) all get through.
func TestRelayAllowsTheExtensionsRequests(t *testing.T) {
	source, err := os.ReadFile("../../extension/src/shared/protocol.ts")
	if err != nil {
		t.Fatal(err)
	}
	block := func(start string) string {
		text := string(source)
		i := strings.Index(text, start)
		if i < 0 {
			t.Fatalf("%q isn't in protocol.ts", start)
		}
		text, _, _ = strings.Cut(text[i:], ";\n")
		return text
	}
	requests := block("export type Request =")
	actions := block("export type ControlAction =")
	var names []string
	for _, m := range regexp.MustCompile(`type: "(\w+)"`).FindAllStringSubmatch(requests, -1) {
		names = append(names, m[1])
	}
	for _, m := range regexp.MustCompile(`"(\w+)"`).FindAllStringSubmatch(actions, -1) {
		names = append(names, m[1])
	}
	if len(names) < 10 {
		t.Fatalf("found only %v in protocol.ts", names)
	}
	for _, name := range names {
		if !allowed[name] {
			t.Errorf("the extension's %q request isn't allowed", name)
		}
	}
}

func TestRelay(t *testing.T) {
	addr := testAddress(t)
	app := startFakeApp(t, addr)
	b := startBrowser(t, &Relay{Socket: addr, StartApp: func() error { return errors.New("should be running") }})

	b.expect(t, `"type":"ready"`)
	b.send(t, `{"type":"ping","rid":1}`)
	b.expect(t, `"ok":true,"rid":1`)

	b.send(t, `{"type":"config","rid":2,"set":{"download_dir":"/"}}`)
	b.expect(t, `"error":"not allowed","ok":false,"rid":2`)
	if got := app.received(); strings.Contains(got, "config") {
		t.Errorf("a settings change reached the app: %s", got)
	}

	app.push(`{"type":"update","download":{"id":"x"}}`)
	b.expect(t, `"type":"update"`)

	app.closeAll()
	b.expect(t, `"type":"daemon_gone"`)
	if err := b.wait(t); err != nil {
		t.Errorf("Run = %v", err)
	}
}

func TestRelayStartsTheApp(t *testing.T) {
	addr := testAddress(t)
	started := false
	b := startBrowser(t, &Relay{Socket: addr, StartApp: func() error {
		started = true
		go func() {
			time.Sleep(300 * time.Millisecond) // apps take a moment to listen
			startFakeApp(t, addr)
		}()
		return nil
	}})
	b.expect(t, `"type":"ready"`)
	if !started {
		t.Error("the app wasn't started")
	}
}

func TestRelaySaysWhenTheAppWontStart(t *testing.T) {
	b := startBrowser(t, &Relay{Socket: testAddress(t), StartApp: func() error { return nil }, StartTimeout: 500 * time.Millisecond})
	b.expect(t, `"type":"error"`)
	if err := b.wait(t); err == nil {
		t.Error("Run succeeded without an app")
	}
}

type fakeApp struct {
	listener net.Listener
	conns    chan *ipc.Conn
	got      chan string
}

func startFakeApp(t *testing.T, addr string) *fakeApp {
	l, err := ipc.Listen(addr)
	if err != nil {
		t.Error(err)
		return nil
	}
	a := &fakeApp{listener: l, conns: make(chan *ipc.Conn, 10), got: make(chan string, 100)}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			conn := ipc.NewConn(c)
			a.conns <- conn
			go func() {
				for {
					message, err := conn.Receive()
					if err != nil {
						return
					}
					a.got <- string(message)
					var req struct {
						RID json.RawMessage `json:"rid"`
					}
					_ = json.Unmarshal(message, &req)
					_ = conn.Send(map[string]any{"type": "reply", "rid": req.RID, "ok": true})
				}
			}()
		}
	}()
	return a
}

func (a *fakeApp) push(message string) {
	conn := <-a.conns
	a.conns <- conn
	_ = conn.SendRaw([]byte(message))
}

func (a *fakeApp) received() string {
	var all []string
	for {
		select {
		case m := <-a.got:
			all = append(all, m)
		default:
			return strings.Join(all, "\n")
		}
	}
}

func (a *fakeApp) closeAll() {
	a.listener.Close()
	for {
		select {
		case c := <-a.conns:
			c.Close()
		default:
			return
		}
	}
}

type browser struct {
	in     *io.PipeWriter
	out    *io.PipeReader
	frames chan string
	done   chan error
}

func startBrowser(t *testing.T, r *Relay) *browser {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	r.FromBrowser, r.ToBrowser = inR, outW
	b := &browser{in: inW, out: outR, frames: make(chan string, 100), done: make(chan error, 1)}
	go func() { b.done <- r.Run(); outW.Close() }()
	go func() {
		for {
			frame, err := readFrame(outR)
			if err != nil {
				close(b.frames)
				return
			}
			b.frames <- string(frame)
		}
	}()
	t.Cleanup(func() { inW.Close() })
	return b
}

func (b *browser) send(t *testing.T, message string) {
	if err := writeFrame(b.in, []byte(message)); err != nil {
		t.Fatal(err)
	}
}

func (b *browser) expect(t *testing.T, part string) {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case frame, ok := <-b.frames:
			if !ok {
				t.Fatalf("no message with %s before the relay ended", part)
			}
			if strings.Contains(frame, part) {
				return
			}
		case <-timeout:
			t.Fatalf("no message with %s", part)
		}
	}
}

func (b *browser) wait(t *testing.T) error {
	select {
	case err := <-b.done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("the relay didn't end")
		return nil
	}
}

func testAddress(t *testing.T) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(`\\.\pipe\snag-test-%d-%s`, os.Getpid(), t.Name())
	}
	dir, err := os.MkdirTemp("", "snag")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "app.sock")
}
