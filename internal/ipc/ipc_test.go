package ipc

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func testAddress(t *testing.T) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(`\\.\pipe\snag-test-%d-%s`, os.Getpid(), strings.ReplaceAll(t.Name(), "/", "-"))
	}
	// Socket paths are short-lived and limited to ~100 bytes, so not under the long test temp dir.
	dir, err := os.MkdirTemp("", "snag")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "app.sock")
}

func TestMessagesBothWays(t *testing.T) {
	addr := testAddress(t)
	l, err := Listen(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		conn := NewConn(c)
		defer conn.Close()
		for {
			message, err := conn.Receive()
			if err != nil {
				return
			}
			_ = conn.SendRaw(message)
		}
	}()

	c, err := Dial(addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	conn := NewConn(c)
	defer conn.Close()
	want := map[string]any{"type": "ping", "rid": 1.0, "text": strings.Repeat("ü", 100_000)}
	if err := conn.Send(want); err != nil {
		t.Fatal(err)
	}
	reply, err := conn.Receive()
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(reply, &got); err != nil || got["text"] != want["text"] || got["rid"] != 1.0 {
		t.Errorf("echo = %.80v, %v", got, err)
	}
}

func TestListenRefusesASecondApp(t *testing.T) {
	addr := testAddress(t)
	l, err := Listen(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	if second, err := Listen(addr); !errors.Is(err, ErrInUse) {
		if second != nil {
			second.Close()
		}
		t.Errorf("second Listen error = %v, want ErrInUse", err)
	}
}

func TestListenReplacesAStaleSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("named pipes vanish with their app")
	}
	addr := testAddress(t)
	if err := os.WriteFile(addr, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := Listen(addr)
	if err != nil {
		t.Fatalf("a stale socket file blocked listening: %v", err)
	}
	l.Close()
}

func TestSocketIsPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the pipe's security descriptor does this on Windows")
	}
	addr := testAddress(t)
	l, err := Listen(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	info, err := os.Stat(addr)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("socket mode %v, %v; want 0600", info.Mode().Perm(), err)
	}
}
