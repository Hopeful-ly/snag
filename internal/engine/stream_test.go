package engine

import (
	"errors"
	"testing"
)

func TestAStreamWithDifferentCaptionsIsANewDownload(t *testing.T) {
	te := newTestEngine(t, Settings{})
	plain := te.add(t, AddRequest{URL: "https://x/watch?v=1", Kind: KindStream, Format: "1080", Paused: true})
	subs, duplicate, err := te.Add(AddRequest{
		URL: plain.URL, Kind: KindStream, Format: "1080", Subs: "en", Paused: true,
	})
	if err != nil || duplicate || subs.ID == plain.ID {
		t.Errorf("the same quality with captions: duplicate %v, err %v", duplicate, err)
	}
	again, duplicate, _ := te.Add(AddRequest{
		URL: plain.URL, Kind: KindStream, Format: "1080", Subs: "en", Paused: true,
	})
	if !duplicate || again.ID != subs.ID {
		t.Error("the same quality and captions again isn't the same download")
	}
}

func TestAStreamOfAnotherQualityIsANewDownload(t *testing.T) {
	te := newTestEngine(t, Settings{})
	hd := te.add(t, AddRequest{URL: "https://x/watch?v=1", Kind: KindStream, Format: "1080", Paused: true})
	audio, duplicate, err := te.Add(AddRequest{URL: hd.URL, Kind: KindStream, Format: "ba", Audio: "mp3", Paused: true})
	if err != nil || duplicate || audio.ID == hd.ID {
		t.Errorf("another quality of the same video: duplicate %v, err %v", duplicate, err)
	}
	again, duplicate, _ := te.Add(AddRequest{URL: hd.URL, Kind: KindStream, Format: "1080", Paused: true})
	if !duplicate || again.ID != hd.ID {
		t.Error("the same quality again isn't the same download")
	}
}

func TestStreamGuessedFromItsAddress(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.add(t, AddRequest{URL: "https://cdn.x/live/index.M3U8?token=1", Paused: true})
	if d.Kind != KindStream {
		t.Errorf("kind = %q, want stream", d.Kind)
	}
}

func TestAStreamUnderANewLinkContinues(t *testing.T) {
	te := newTestEngine(t, Settings{})
	first := AddRequest{URL: "https://cdn.x/tokA/v.m3u8", Kind: KindStream, Page: "https://x/watch/1",
		Title: "Token test", Label: "720p", Format: "hls-720"}
	d := te.add(t, first)
	te.waitRunning(t, d.ID)
	te.runner.finish(first.URL, errors.New("the stream link expired"))
	te.waitFor(t, d.ID, StatusError)

	renewed := first
	renewed.URL = "https://cdn.x/tokB/v.m3u8"
	renewed.Headers = map[string]string{"Referer": "https://x/watch/1"}
	got, duplicate, err := te.Add(renewed)
	if err != nil || !duplicate || got.ID != d.ID {
		t.Fatalf("Add under a new link: %s, duplicate %v, err %v; want the old download", got.ID, duplicate, err)
	}
	got = te.waitFor(t, d.ID, StatusDownloading)
	jobs := te.runner.waitJobs(t, 2)
	if got.URL != renewed.URL || got.Error != "" || jobs[1].URL != renewed.URL || jobs[1].WorkDir != jobs[0].WorkDir {
		t.Errorf("continued as %+v with job %+v", got, jobs[1])
	}

	other := first
	other.URL, other.Label, other.Format = "https://cdn.x/tokC/v.m3u8", "1080p", "hls-1080"
	if _, duplicate, _ := te.Add(other); duplicate {
		t.Error("another quality from the same page took over the download")
	}
}

func TestAStreamsNameLeavesTheContainerToYtDLP(t *testing.T) {
	te := newTestEngine(t, Settings{Ask: true})
	d := te.add(t, AddRequest{URL: "https://x/v.m3u8", Filename: "Holiday.mp4"})
	if d.Filename != "Holiday" {
		t.Errorf("filename = %q, want Holiday", d.Filename)
	}
	if err := te.Confirm(d.ID, ConfirmRequest{Filename: "Trip.mkv", Paused: true}); err != nil {
		t.Fatal(err)
	}
	if got, _ := te.Get(d.ID); got.Filename != "Trip" {
		t.Errorf("confirmed filename = %q, want Trip", got.Filename)
	}
}
