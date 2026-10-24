package app

import (
	"bytes"
	"image/png"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/packaging/icons"
)

func TestViewOf(t *testing.T) {
	created := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	v := viewOf(engine.Download{ID: "a", URL: "https://x/files/Clip.mp4", Kind: engine.KindStream,
		Status: engine.StatusDownloading, Size: 1000, Done: 400, Speed: 100, Created: created})
	if v.Name != "Clip.mp4" || v.Kind != "stream" || v.ETA == nil || *v.ETA != 6 || v.Created != created.UnixMilli() || v.Finished != 0 {
		t.Errorf("viewOf = %+v", v)
	}
	if noted := viewOf(engine.Download{Note: "Saved without subtitles"}); noted.Note != "Saved without subtitles" {
		t.Errorf("note = %q", noted.Note)
	}
	if idle := viewOf(engine.Download{Size: 1000}); idle.ETA != nil {
		t.Errorf("no speed, yet an eta of %d", *idle.ETA)
	}
}

func TestSizeText(t *testing.T) {
	for bytes, want := range map[int64]string{
		0: "0 B", 900: "900 B", 2048: "2 KB", 5 << 20: "5.0 MB", 25 << 20: "25 MB", 3 << 30: "3.0 GB",
	} {
		if got := sizeText(bytes); got != want {
			t.Errorf("sizeText(%d) = %q, want %q", bytes, got, want)
		}
	}
}

func TestTrayLine(t *testing.T) {
	d := engine.Download{Path: "/dl/A rather long name for a film that goes on and on.mp4", Size: 200, Done: 90, Speed: 2 << 20}
	if got, want := trayLine(d), "A rather long name for a film that goes… — 45% · 2.0 MB/s"; got != want {
		t.Errorf("trayLine = %q, want %q", got, want)
	}
}

func TestBusyIconHasADot(t *testing.T) {
	plain, err := png.Decode(bytes.NewReader(icons.Tray))
	if err != nil {
		t.Fatal(err)
	}
	busy, err := png.Decode(bytes.NewReader(trayBusyIcon))
	if err != nil {
		t.Fatalf("busy icon: %v", err)
	}
	b := busy.Bounds()
	corner := busy.At(b.Max.X-b.Dx()/5-1, b.Max.Y-b.Dy()/5-1)
	if r, g, _, _ := corner.RGBA(); g>>8 < 0xc0 || r>>8 > 0x80 {
		t.Errorf("no green dot in the corner: %v", corner)
	}
	if plain.Bounds() != b {
		t.Error("the busy icon changed size")
	}
}
