//go:build !windows

package ipc

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Listen opens addr, a socket path, for connections. A socket file left by an app that crashed
// is replaced; one with an app behind it gives ErrInUse.
func Listen(addr string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(addr), 0o700); err != nil {
		return nil, err
	}
	if _, err := os.Stat(addr); err == nil {
		if c, err := net.DialTimeout("unix", addr, time.Second); err == nil {
			c.Close()
			return nil, ErrInUse
		}
		if err := os.Remove(addr); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	l, err := net.Listen("unix", addr)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(addr, 0o600); err != nil {
		l.Close()
		return nil, err
	}
	return l, nil
}

// Dial connects to the app listening on addr; a timeout of 0 means none.
func Dial(addr string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", addr, timeout)
}
