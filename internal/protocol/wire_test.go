package protocol

import (
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/engine"
)

func TestToWire(t *testing.T) {
	created := time.Date(2026, 10, 7, 12, 0, 0, 500_000_000, time.UTC)
	w := toWire(engine.Download{Kind: engine.KindStream, Size: 1000, Done: 400, Speed: 100, Created: created})
	if w.Kind != "ytdlp" || w.ETA == nil || *w.ETA != 6 || w.Created != float64(created.UnixMilli())/1000 || w.Finished != 0 {
		t.Errorf("toWire = %+v", w)
	}
	if idle := toWire(engine.Download{Kind: engine.KindHTTP, Size: 1000}); idle.ETA != nil || idle.Kind != "http" {
		t.Errorf("no speed, no eta: %+v", idle)
	}
	for wire, want := range map[string]engine.Kind{"http": engine.KindHTTP, "ytdlp": engine.KindStream, "": ""} {
		if got, ok := kindFromWire(wire); !ok || got != want {
			t.Errorf("kindFromWire(%q) = %q, %v", wire, got, ok)
		}
	}
	if _, ok := kindFromWire("torrent"); ok {
		t.Error("an unknown kind was accepted")
	}
}
