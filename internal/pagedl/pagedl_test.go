package pagedl

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/meta"
)

type next struct{ jobs []engine.Job }

func (n *next) Run(_ context.Context, job engine.Job, _ engine.Reporter) error {
	n.jobs = append(n.jobs, job)
	return nil
}

type resolver struct {
	src   Source
	err   error
	pages []string
}

func (r *resolver) Resolve(_ context.Context, req Request) (Source, error) {
	r.pages = append(r.pages, req.Page)
	return r.src, r.err
}

func TestRunResolvesEveryRun(t *testing.T) {
	stream := &next{}
	found := &resolver{src: Source{URL: "https://cdn.example/v/index.m3u8", Headers: map[string]string{"Referer": "https://site.example/"}}}
	r := Runner{Resolver: found, Next: stream}
	job := engine.Job{URL: "https://site.example/tv/1/1/2?play=true", Resolve: true, Title: "Episode 2"}
	for range 2 { // a resume must resolve again
		if err := r.Run(context.Background(), job, nil); err != nil {
			t.Fatal(err)
		}
	}
	if len(found.pages) != 2 || found.pages[0] != job.URL {
		t.Errorf("resolved %v", found.pages)
	}
	got := stream.jobs[1]
	if got.URL != found.src.URL || got.Resolve || got.Headers["Referer"] != "https://site.example/" || got.Title != "Episode 2" {
		t.Errorf("the stream runner got %+v", got)
	}
}

func TestRunPassesOtherJobsOn(t *testing.T) {
	stream := &next{}
	if err := (Runner{Next: stream}).Run(context.Background(), engine.Job{URL: "https://cdn.example/a.m3u8"}, nil); err != nil {
		t.Fatal(err)
	}
	if len(stream.jobs) != 1 {
		t.Error("a plain stream wasn't passed on")
	}
}

func TestRunFailures(t *testing.T) {
	job := engine.Job{URL: "https://site.example/tv/1/1/2", Resolve: true}
	if err := (Runner{Next: &next{}}).Run(context.Background(), job, nil); !errors.Is(err, ErrNoBrowser) {
		t.Errorf("without a resolver: %v", err)
	}
	failed := errors.New("couldn't find “Song” on YouTube Music")
	r := Runner{Resolver: &resolver{err: failed}, Next: &next{}}
	if err := r.Run(context.Background(), job, nil); !errors.Is(err, failed) || err.Error() != failed.Error() {
		t.Errorf("a failed resolve: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r = Runner{Resolver: &resolver{err: context.Canceled}, Next: &next{}}
	if err := r.Run(ctx, job, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("paused while resolving: %v", err)
	}
}

func TestBrowserError(t *testing.T) {
	page := "https://site.example/tv/1/1/2"
	if err := BrowserError(page, errors.New("no video started playing")); err == nil ||
		err.Error() != "couldn't find the video on site.example: no video started playing" {
		t.Errorf("a failure: %v", err)
	}
	if err := BrowserError(page, context.DeadlineExceeded); err == nil || !strings.Contains(err.Error(), "didn't start playing") {
		t.Errorf("too long: %v", err)
	}
	if err := BrowserError(page, ErrNoBrowser); !errors.Is(err, ErrNoBrowser) {
		t.Errorf("no browser: %v", err)
	}
}

func TestFoundTagsFillTheJobs(t *testing.T) {
	stream := &next{}
	found := &resolver{src: Source{URL: "https://music.example/watch?v=1",
		Tags: meta.Tags{Title: "master", Cover: "https://i.example/c.jpg", Date: "2013"}}}
	job := engine.Job{URL: "https://open.spotify.com/track/x", Resolve: true,
		Meta: meta.Label{Tags: meta.Tags{Title: "Get Lucky", Artist: "Daft Punk"}}}
	if err := (Runner{Resolver: found, Next: stream}).Run(context.Background(), job, nil); err != nil {
		t.Fatal(err)
	}
	want := meta.Tags{Title: "Get Lucky", Artist: "Daft Punk", Cover: "https://i.example/c.jpg", Date: "2013"}
	if got := stream.jobs[0].Meta.Tags; got != want {
		t.Errorf("tags %+v, want %+v", got, want)
	}
}
