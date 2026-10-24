package protocol

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/Roham1299/snag/internal/ipc"
	"github.com/Roham1299/snag/internal/pagedl"
)

type pendingResolve struct {
	conn   *ipc.Conn
	answer chan resolveAnswer // buffered 1
}

type resolveAnswer struct {
	src pagedl.Source
	err error
}

// Resolve asks a browser with Snag's extension (the one that asked something last, else any) to
// play page and report the stream it plays. It implements pagedl.Resolver.
func (h *Handler) Resolve(ctx context.Context, page string) (pagedl.Source, error) {
	token := newToken()
	pending := &pendingResolve{answer: make(chan resolveAnswer, 1)}
	h.mu.Lock()
	pending.conn = h.recent
	for c := range h.subscribers {
		if pending.conn == nil {
			pending.conn = c
		}
	}
	if pending.conn != nil {
		h.resolving[token] = pending
	}
	h.mu.Unlock()
	if pending.conn == nil {
		return pagedl.Source{}, pagedl.ErrNoBrowser
	}
	defer func() {
		h.mu.Lock()
		delete(h.resolving, token)
		h.mu.Unlock()
	}()
	if err := pending.conn.Send(map[string]any{"type": "resolve", "token": token, "page": page}); err != nil {
		return pagedl.Source{}, pagedl.ErrNoBrowser
	}
	select {
	case a := <-pending.answer:
		return a.src, a.err
	case <-ctx.Done():
		return pagedl.Source{}, ctx.Err()
	}
}

func (h *Handler) resolved(req request) error {
	h.mu.Lock()
	pending := h.resolving[req.Token]
	delete(h.resolving, req.Token)
	h.mu.Unlock()
	if pending == nil {
		return errors.New("nothing asked for that")
	}
	var a resolveAnswer
	switch {
	case req.Type == "resolve_failed":
		a.err = errors.New(req.Error)
		if req.Error == "" {
			a.err = errors.New("no video played")
		}
	case !isWebAddress(req.URL):
		a.err = errors.New("the browser found no stream to download")
	default:
		a.src = pagedl.Source{URL: req.URL, Headers: req.Headers, Cookies: req.Cookies}
	}
	pending.answer <- a
	return nil
}

// browserGone fails the pages a browser that disconnected was finding streams on.
func (h *Handler) browserGone(conn *ipc.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for token, pending := range h.resolving {
		if pending.conn == conn {
			delete(h.resolving, token)
			pending.answer <- resolveAnswer{err: pagedl.ErrNoBrowser}
		}
	}
}

func newToken() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
