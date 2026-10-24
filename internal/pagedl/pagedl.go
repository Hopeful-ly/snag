// Package pagedl downloads videos whose stream only a browser or search can find. Each run
// resolves the stream afresh, since such links expire.
package pagedl

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/Roham1299/snag/internal/cookies"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/meta"
)

// ErrNoBrowser means no browser with Snag's extension is connected.
var ErrNoBrowser = errors.New("open your browser with Snag's extension to fetch this video, then retry")

const resolveTimeout = 60 * time.Second

// Source is a page's resolved stream.
type Source struct {
	URL     string
	Headers map[string]string
	Cookies []cookies.Cookie
	// Tags fill only what the job's own tags leave empty.
	Tags meta.Tags
}

// Request asks for a page's stream.
type Request struct {
	Page string
	// Fallback is "youtube" or "soundcloud" for when YouTube Music has no match; "" asks the user.
	Fallback string
}

// Resolver finds the stream a page plays. Its errors are shown to the user verbatim.
type Resolver interface {
	Resolve(ctx context.Context, req Request) (Source, error)
}

// Runner resolves jobs marked Resolve and passes every job on to Next.
type Runner struct {
	Resolver Resolver // nil: jobs to resolve fail with ErrNoBrowser
	Next     engine.Runner
}

// Run implements engine.Runner.
func (r Runner) Run(ctx context.Context, job engine.Job, report engine.Reporter) error {
	if !job.Resolve {
		return r.Next.Run(ctx, job, report)
	}
	if r.Resolver == nil {
		return ErrNoBrowser
	}
	resolveCtx, cancel := context.WithTimeout(ctx, resolveTimeout)
	src, err := r.Resolver.Resolve(resolveCtx, Request{Page: job.URL, Fallback: job.Fallback})
	cancel()
	switch {
	case ctx.Err() != nil:
		return ctx.Err()
	case err != nil:
		return err
	}
	job.Resolve = false
	job.URL, job.Headers, job.Cookies = src.URL, src.Headers, src.Cookies
	job.Meta.Tags = job.Meta.Tags.Or(src.Tags)
	return r.Next.Run(ctx, job, report)
}

// BrowserError explains why the browser didn't find page's stream.
func BrowserError(page string, err error) error {
	switch {
	case err == nil, errors.Is(err, ErrNoBrowser), errors.Is(err, context.Canceled):
		return err
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("the video on %s didn't start playing in the browser", hostOf(page))
	}
	return fmt.Errorf("couldn't find the video on %s: %w", hostOf(page), err)
}

func hostOf(address string) string {
	if u, err := url.Parse(address); err == nil && u.Host != "" {
		return u.Host
	}
	return "the page"
}
