// Package ipc connects the browser helper and the app: newline-delimited JSON over a Unix
// socket (a named pipe on Windows) that only the current user can open.
package ipc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"sync"
)

// MaxMessage is the largest message accepted.
const MaxMessage = 4 << 20

// ErrInUse means another app already listens on the address.
var ErrInUse = errors.New("another Snag is already running")

// ErrTooLong means a message was larger than MaxMessage.
var ErrTooLong = errors.New("message too long")

// Conn is one connection. Send may be called from several goroutines; Receive from one.
type Conn struct {
	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex
}

// NewConn wraps c.
func NewConn(c net.Conn) *Conn {
	return &Conn{conn: c, reader: bufio.NewReaderSize(c, 64<<10)}
}

// Receive returns the next message.
func (c *Conn) Receive() (json.RawMessage, error) {
	var line []byte
	for {
		part, isPrefix, err := c.reader.ReadLine()
		if err != nil {
			return nil, err
		}
		line = append(line, part...)
		if len(line) > MaxMessage {
			return nil, ErrTooLong
		}
		if !isPrefix {
			if len(bytes.TrimSpace(line)) == 0 {
				line = line[:0]
				continue
			}
			return line, nil
		}
	}
}

// Send writes v as one message.
func (c *Conn) Send(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.SendRaw(data)
}

// SendRaw writes a message that is already JSON.
func (c *Conn) SendRaw(message []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.conn.Write(append(bytes.TrimSpace(message), '\n'))
	return err
}

// Close closes the connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}
