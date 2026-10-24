package httpdl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Roham1299/snag/internal/cookies"
	"github.com/Roham1299/snag/internal/engine"
)

// UserAgent is a browser user agent, since sites treat unknown clients differently.
const UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Safari/537.36"

// StatusError is a server's refusal.
type StatusError struct {
	Code int
}

// Error describes the refusal.
func (e *StatusError) Error() string {
	return fmt.Sprintf("server said %d %s", e.Code, http.StatusText(e.Code))
}

var (
	errNoResume    = errors.New("server stopped allowing resume")
	errClosedEarly = errors.New("connection closed early")
	errStalled     = errors.New("connection stalled")
)

// diskError is a write failure, which retrying won't fix.
type diskError struct{ err error }

func (e *diskError) Error() string { return "can't write the file: " + e.err.Error() }
func (e *diskError) Unwrap() error { return e.err }

func isPermanent(err error) bool {
	var status *StatusError
	if errors.As(err, &status) {
		switch status.Code {
		case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusGone:
			return true
		}
	}
	var disk *diskError
	return errors.As(err, &disk) || errors.Is(err, errNoResume)
}

// get requests bytes from..to inclusive; to < 0 means to the end.
func (d *download) get(ctx context.Context, from, to int64) (*http.Response, error) {
	reqCtx, cancel := context.WithCancel(ctx)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, d.url, nil)
	if err != nil {
		cancel()
		return nil, err
	}
	setHeaders(req, d.job)
	byteRange := fmt.Sprintf("bytes=%d-", from)
	if to >= 0 {
		byteRange += strconv.FormatInt(to, 10)
	}
	req.Header.Set("Range", byteRange)

	resp, err := d.runner.client.Do(req)
	if err != nil {
		cancel()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return nil, fmt.Errorf("can't reach server: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		resp.Body.Close()
		cancel()
		return nil, &StatusError{Code: resp.StatusCode}
	}
	resp.Body = newStallGuard(resp.Body, cancel)
	return resp, nil
}

func setHeaders(req *http.Request, job engine.Job) {
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "*/*")
	// Ranges must count bytes of the file itself, not of a compressed form of it.
	req.Header.Set("Accept-Encoding", "identity")
	for name, value := range job.Headers {
		switch strings.ToLower(name) {
		case "range", "accept-encoding", "host", "content-length", "connection":
			continue
		}
		req.Header.Set(name, value)
	}
	if cookie := cookies.Header(job.Cookies, req.URL); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
}

// contentLength returns the full size from a ranged response (0 when unknown) and whether the
// server honored the range.
func contentLength(resp *http.Response) (size int64, ranged bool) {
	if resp.StatusCode == http.StatusPartialContent {
		// Content-Range: bytes 0-99/1234
		if _, total, ok := strings.Cut(resp.Header.Get("Content-Range"), "/"); ok {
			if n, err := strconv.ParseInt(strings.TrimSpace(total), 10, 64); err == nil && n > 0 {
				return n, true
			}
		}
		return 0, false
	}
	return max(resp.ContentLength, 0), false
}

// stallGuard cancels a response whose body sends nothing for stallTimeout, so it is retried.
type stallGuard struct {
	body    io.ReadCloser
	timer   *time.Timer
	cancel  context.CancelFunc
	stalled atomic.Bool
}

func newStallGuard(body io.ReadCloser, cancel context.CancelFunc) *stallGuard {
	g := &stallGuard{body: body, cancel: cancel}
	g.timer = time.AfterFunc(stallTimeout, func() {
		g.stalled.Store(true)
		cancel()
	})
	return g
}

func (g *stallGuard) Read(p []byte) (int, error) {
	n, err := g.body.Read(p)
	if n > 0 {
		g.timer.Reset(stallTimeout)
	}
	if err != nil && g.stalled.Load() {
		err = errStalled
	}
	return n, err
}

func (g *stallGuard) Close() error {
	g.timer.Stop()
	g.cancel()
	return g.body.Close()
}
