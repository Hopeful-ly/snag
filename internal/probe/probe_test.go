package probe

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/streamdl/streamtest"
	"github.com/Roham1299/snag/internal/tools"
	"github.com/Roham1299/snag/internal/ytdlp"
)

func TestPageChoices(t *testing.T) {
	info := ytdlp.Info{Duration: 100, Formats: []ytdlp.Format{
		{FormatID: "140", Ext: "m4a", VCodec: "none", ACodec: "mp4a", ABR: 128, Filesize: 1_600_000},
		{FormatID: "251", Ext: "webm", VCodec: "none", ACodec: "opus", ABR: 160, Filesize: 2_000_000},
		{FormatID: "299", Ext: "mp4", VCodec: "avc1", ACodec: "none", Height: 1080, FPS: 60, Filesize: 50_000_000},
		{FormatID: "137", Ext: "mp4", VCodec: "avc1", ACodec: "none", Height: 1080, FPS: 30, Filesize: 30_000_000},
		{FormatID: "248", Ext: "webm", VCodec: "vp9", ACodec: "none", Height: 1080, FPS: 30, Filesize: 25_000_000},
		{FormatID: "337", Ext: "webm", VCodec: "vp9", ACodec: "none", Height: 2160, FPS: 60, DynamicRange: "HDR10"},
		{FormatID: "22", Ext: "mp4", VCodec: "avc1", ACodec: "mp4a", Height: 720, FPS: 30, TBR: 1000},
		{FormatID: "sb0", Ext: "mhtml", VCodec: "none", ACodec: "none"},
	}}
	got := pageChoices(info)
	want := []struct {
		label, detail string
		size          int64
	}{
		{"2160p60", "WEBM · HDR10", 2_000_000}, // no stated size: only its audio's counts
		{"1080p60", "MP4", 52_000_000},
		{"1080p", "MP4", 32_000_000}, // MP4 is preferred over a smaller WebM
		{"720p", "MP4", 12_500_000},  // has its own audio; size from its bitrate
		{"Audio", "M4A", 2_000_000},
		{"Audio", "MP3", 2_000_000},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d choices, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Label != w.label || got[i].Detail != w.detail || got[i].Size != w.size {
			t.Errorf("choice %d = %s %q %d, want %s %q %d", i, got[i].Label, got[i].Detail, got[i].Size, w.label, w.detail, w.size)
		}
	}
	if f := got[1].Format; !strings.HasPrefix(f, "bv*[height=1080][fps>30]+ba") {
		t.Errorf("1080p60 format selector = %q", f)
	}
	if !got[5].AudioOnly || got[5].Audio != "mp3" || got[4].Audio != "m4a" {
		t.Error("audio choices aren't marked")
	}
}

func TestStreamChoices(t *testing.T) {
	info := ytdlp.Info{Formats: []ytdlp.Format{
		{FormatID: "hls-2000", Ext: "mp4", Height: 720, TBR: 2000, URL: "https://x/mid2.m3u8"},
		{FormatID: "hls-5000", Ext: "mp4", Height: 1080, TBR: 5000, FPS: 50, URL: "https://x/hi.m3u8"},
		{FormatID: "hls-2500", Ext: "mp4", Height: 720, TBR: 2500, URL: "https://x/mid.m3u8"},
	}}
	got := streamChoices(info, "https://x/master.m3u8")
	labels := []string{got[0].Label, got[1].Label, got[2].Label}
	if !slices.Equal(labels, []string{"1080p", "720p", "720p"}) || got[1].Detail != "MP4 · 2.5 Mbps" {
		t.Errorf("choices = %+v", got)
	}
	if got[0].Detail != "MP4 · 5.0 Mbps · 50 fps" || got[0].Format != "hls-5000+ba/hls-5000" || got[0].playlist != "https://x/hi.m3u8" {
		t.Errorf("first choice = %+v", got[0])
	}

	single := streamChoices(ytdlp.Info{Duration: 600}, "https://cdn.x/v/index-s1080p-v1.m3u8")
	if len(single) != 1 || single[0].Label != "1080p" || single[0].Detail != "MP4 · 10 min" {
		t.Errorf("a single stream named by its address: %+v", single)
	}
}

func TestSniff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hls":
			fmt.Fprint(w, "\uFEFF\n#EXTM3U\n#EXT-X-VERSION:3\n")
		case "/dash":
			fmt.Fprint(w, `<?xml version="1.0"?><MPD xmlns="urn:mpeg:dash:schema:mpd:2011">`)
		default:
			fmt.Fprint(w, "<html>a page</html>")
		}
	}))
	defer srv.Close()
	p := New(tools.Paths{})
	for path, want := range map[string]string{"/hls": KindHLS, "/dash": KindDASH, "/page": ""} {
		if got := p.Sniff(context.Background(), srv.URL+path, nil, nil); got.Kind != want {
			t.Errorf("Sniff(%s) = %q, want %q", path, got.Kind, want)
		}
	}
}

func TestEstimateHLSSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ranges.m3u8":
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-BYTERANGE:1000@0\na.ts\n#EXT-X-BYTERANGE:2500@1000\na.ts\n")
		case "/pieces.m3u8":
			fmt.Fprint(w, "#EXTM3U\n")
			for i := range 8 {
				fmt.Fprintf(w, "#EXTINF:4,\npiece%d.ts\n", i)
			}
		case "/master.m3u8":
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1\nhi.m3u8\n")
		default:
			http.ServeContent(w, r, "piece.ts", srvTime, strings.NewReader(strings.Repeat("x", 3000)))
		}
	}))
	defer srv.Close()
	p := New(tools.Paths{})
	for path, want := range map[string]int64{"/ranges.m3u8": 3500, "/pieces.m3u8": 8 * 3000, "/master.m3u8": 0} {
		if got := p.estimateHLSSize(context.Background(), srv.URL+path, nil, nil); got != want {
			t.Errorf("estimateHLSSize(%s) = %d, want %d", path, got, want)
		}
	}
}

func TestProbeAMasterPlaylist(t *testing.T) {
	p := newProber(t)
	srv := streamtest.NewServer(t)
	result := p.Probe(context.Background(), Request{URL: srv.URL + "/hls/master.m3u8"})
	if !result.Supported {
		t.Fatalf("not supported: %q", result.Reason)
	}
	var labels []string
	for _, c := range result.Choices {
		labels = append(labels, c.Label)
	}
	if !slices.Equal(labels, []string{"1080p", "720p", "720p"}) || len(result.Variants) != 3 {
		t.Errorf("labels %v and %d variants; want every variant", labels, len(result.Variants))
	}
	// The playlist states 5 Mbps; the stream is 200 pieces of 4 s.
	if want := int64(5000 * 1000 / 8 * 800); result.Choices[0].Size != want {
		t.Errorf("size from the bitrate %d, want %d", result.Choices[0].Size, want)
	}
}

func TestProbeEstimatesAStreamThatStatesNoSize(t *testing.T) {
	p := newProber(t)
	srv := streamtest.NewServer(t)
	result := p.Probe(context.Background(), Request{URL: srv.URL + "/single/v.m3u8"})
	if !result.Supported || len(result.Choices) != 1 {
		t.Fatalf("result = %+v", result)
	}
	if want := int64(srv.Pieces * srv.PieceSize); result.Choices[0].Size != want {
		t.Errorf("estimated size %d, want %d (from sampled pieces)", result.Choices[0].Size, want)
	}
}

func TestProbeAPageWithoutVideo(t *testing.T) {
	p := newProber(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html><body>nothing here</body></html>")
	}))
	defer srv.Close()
	if result := p.Probe(context.Background(), Request{URL: srv.URL + "/article"}); result.Supported || result.Reason != "" {
		t.Errorf("result = %+v, want unsupported with no reason", result)
	}
}

var srvTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func newProber(t *testing.T) *Prober {
	t.Helper()
	programs := tools.Find("")
	if programs.YtDLP == "" {
		t.Skip("needs yt-dlp: run `task tools`, or set SNAG_TOOLS")
	}
	return New(programs)
}
