package app

import (
	"testing"

	"github.com/Roham1299/snag/internal/clip"
	"github.com/Roham1299/snag/internal/engine"
)

func TestWatchState(t *testing.T) {
	film := engine.Download{Kind: engine.KindHTTP, Filename: "film.mp4", Size: 100 << 20, Status: engine.StatusDownloading}
	seconds := func(n int64) *int64 { return &n }
	for _, tc := range []struct {
		name  string
		edit  func(*engine.Download)
		state string
		in    *int64
	}{
		{"its start is in", func(d *engine.Download) { d.Playable = 3 << 20 }, "ready", nil},
		{"paused with its start in", func(d *engine.Download) { d.Playable = 3 << 20; d.Status = engine.StatusPaused }, "ready", nil},
		{"on its way", func(d *engine.Download) { d.Playable = 1 << 20; d.Speed = 512 << 10 }, "soon", seconds(2)},
		{"on its way over 8 connections", func(d *engine.Download) {
			d.Playable, d.Speed, d.Connections = 1<<20, 512<<10, 8
		}, "soon", seconds(2)},
		{"not started", func(d *engine.Download) { d.Status = engine.StatusQueued }, "soon", nil},
		{"a small video, all in", func(d *engine.Download) { d.Size, d.Playable = 1000, 1000 }, "ready", nil},
		{"finished: opened instead", func(d *engine.Download) { d.Status = engine.StatusDone }, "", nil},
		{"waiting for a folder", func(d *engine.Download) { d.Status = engine.StatusPending }, "", nil},
		{"not a video", func(d *engine.Download) { d.Filename = "setup.zip" }, "", nil},
		{"size unknown", func(d *engine.Download) { d.Size = 0 }, "", nil},
		{"a site's video with its start in", func(d *engine.Download) {
			d.Kind, d.Estimated, d.Playable = engine.KindStream, true, 1<<20
		}, "ready", nil},
		{"a site's video waiting for its sound", func(d *engine.Download) {
			d.Kind, d.Estimated, d.Speed = engine.KindStream, true, 1<<20
		}, "soon", seconds(1)},
		{"a site's video still fetching its sound", func(d *engine.Download) {
			d.Kind, d.Estimated, d.Speed, d.ToPlay = engine.KindStream, true, 240<<10, 24<<20
		}, "soon", seconds(103)},
		{"a clip, cut as it downloads", func(d *engine.Download) {
			d.Kind, d.Estimated, d.Speed, d.Clip = engine.KindStream, true, 1<<20, clip.Range{From: 40, To: 70}
		}, "", nil},
		{"a site's sound alone", func(d *engine.Download) {
			d.Kind, d.Label, d.Playable = engine.KindStream, "Audio M4A", 5<<20
		}, "", nil},
		{"converted to MP3", func(d *engine.Download) {
			d.Kind, d.Audio, d.Playable = engine.KindStream, "mp3", 5<<20
		}, "", nil},
	} {
		d := film
		tc.edit(&d)
		state, in := watchState(d)
		if state != tc.state || (in == nil) != (tc.in == nil) || (in != nil && *in != *tc.in) {
			t.Errorf("%s: %q, %v; want %q, %v", tc.name, state, deref(in), tc.state, deref(tc.in))
		}
	}
}

func deref(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}
