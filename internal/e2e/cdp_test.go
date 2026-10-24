package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// cdp is a minimal Chrome DevTools Protocol client for one page.
type cdp struct {
	conn *websocket.Conn

	mu      sync.Mutex
	nextID  int
	waiting map[int]chan cdpMessage
	events  []cdpMessage
}

type cdpMessage struct {
	ID     int             `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// attach connects to an open tab, preferring one whose address contains match (Vivaldi opens
// its welcome page first, and its new tabs don't load).
func attach(t *testing.T, port int, match string) *cdp {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		pages := pageTargets(t, port)
		pick := ""
		for url, ws := range pages {
			if strings.Contains(url, match) {
				pick = ws
			}
		}
		if pick == "" && time.Now().After(deadline) {
			for _, ws := range pages {
				pick = ws
			}
		}
		if pick != "" {
			conn, _, err := websocket.Dial(context.Background(), pick, nil) //nolint:bodyclose // the library closes the handshake response
			if err != nil {
				t.Fatal(err)
			}
			conn.SetReadLimit(64 << 20)
			c := &cdp{conn: conn, waiting: map[int]chan cdpMessage{}}
			go c.read()
			t.Cleanup(func() { _ = conn.CloseNow() })
			return c
		}
		if time.Now().After(deadline.Add(15 * time.Second)) {
			t.Fatal("the browser has no tab")
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// pageTargets maps the browser's tab addresses to their DevTools sockets.
func pageTargets(t *testing.T, port int) map[string]string {
	var targets []struct {
		Type, URL            string
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json", port))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		t.Fatal(err)
	}
	pages := map[string]string{}
	for _, target := range targets {
		if target.Type == "page" {
			pages[target.URL] = target.WebSocketDebuggerURL
		}
	}
	return pages
}

func (c *cdp) read() {
	for {
		_, data, err := c.conn.Read(context.Background())
		if err != nil {
			return
		}
		var m cdpMessage
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		c.mu.Lock()
		if m.ID != 0 {
			if ch := c.waiting[m.ID]; ch != nil {
				ch <- m
			}
		} else {
			c.events = append(c.events, m)
		}
		c.mu.Unlock()
	}
}

func (c *cdp) call(t *testing.T, method string, params map[string]any) json.RawMessage {
	t.Helper()
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan cdpMessage, 1)
	c.waiting[id] = ch
	c.mu.Unlock()
	data, _ := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := c.conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-ch:
		if m.Error != nil {
			t.Fatalf("%s: %s", method, m.Error.Message)
		}
		return m.Result
	case <-ctx.Done():
		t.Fatalf("%s: no answer", method)
		return nil
	}
}

func (c *cdp) eventsNamed(method string) []json.RawMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []json.RawMessage
	for _, e := range c.events {
		if e.Method == method {
			out = append(out, e.Params)
		}
	}
	return out
}

// evaluate runs JavaScript in an execution context (0 for the page's own), awaiting promises.
func (c *cdp) evaluate(t *testing.T, context int, expression string, into any) {
	t.Helper()
	params := map[string]any{"expression": expression, "returnByValue": true, "awaitPromise": true}
	if context != 0 {
		params["contextId"] = context
	}
	var result struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text      string `json:"text"`
			Exception struct {
				Description string `json:"description"`
			} `json:"exception"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(c.call(t, "Runtime.evaluate", params), &result); err != nil {
		t.Fatal(err)
	}
	if result.ExceptionDetails != nil {
		t.Fatalf("%s: %s %s", expression, result.ExceptionDetails.Text, result.ExceptionDetails.Exception.Description)
	}
	if into != nil {
		if err := json.Unmarshal(result.Result.Value, into); err != nil {
			t.Fatalf("%s returned %s: %v", expression, result.Result.Value, err)
		}
	}
}
