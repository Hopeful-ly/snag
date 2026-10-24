package engine

import (
	"errors"
	"testing"

	"github.com/Roham1299/snag/internal/clip"
)

func TestAClipIsCutByTheStreamRunnerAndNamedForItsPart(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.add(t, AddRequest{
		URL: "https://cdn.example.com/film.mp4", Kind: KindHTTP, Filename: "Film.mp4",
		Clip: clip.Range{From: 62, To: 98.4}, Paused: true,
	})
	if d.Kind != KindStream || d.Filename != "Film (1.02-1.38)" {
		t.Errorf("clip of a file: kind %q, name %q", d.Kind, d.Filename)
	}
	titled := te.add(t, AddRequest{
		URL: "https://youtube.com/watch?v=a", Kind: KindStream, Title: "Cold Harbor: Part 1",
		Clip: clip.Range{From: 1, To: 5}, Paused: true,
	})
	if titled.Filename != "Cold Harbor Part 1 (0.01-0.05)" {
		t.Errorf("clip named after the title: %q", titled.Filename)
	}
}

func TestAClipIsNotTheWholeVideo(t *testing.T) {
	te := newTestEngine(t, Settings{})
	whole := te.add(t, AddRequest{URL: "https://youtube.com/watch?v=a", Kind: KindStream, Format: "137+140", Paused: true})
	part, duplicate, err := te.Add(AddRequest{
		URL: whole.URL, Kind: KindStream, Format: "137+140", Clip: clip.Range{From: 10, To: 20}, Paused: true,
	})
	if err != nil || duplicate || part.ID == whole.ID {
		t.Fatalf("a clip of a video in the list was taken for it: duplicate %v, err %v", duplicate, err)
	}
	if got, ok := te.Have(HaveQuery{URL: whole.URL, Format: "137+140"}); !ok || got.ID != whole.ID {
		t.Errorf("have the whole video: %+v, %v", got, ok)
	}
	if got, ok := te.Have(HaveQuery{URL: whole.URL, Format: "137+140", Clip: clip.Range{From: 10, To: 20}}); !ok || got.ID != part.ID {
		t.Errorf("have the clip: %+v, %v", got, ok)
	}
	if _, ok := te.Have(HaveQuery{URL: whole.URL, Format: "137+140", Clip: clip.Range{From: 10, To: 30}}); ok {
		t.Error("another clip counts as had")
	}
}

func TestAddRefusesAClipThatEndsBeforeItStarts(t *testing.T) {
	te := newTestEngine(t, Settings{})
	for _, c := range []clip.Range{{From: 5, To: 5}, {From: 8, To: 3}, {From: -1, To: 4}} {
		_, _, err := te.Add(AddRequest{URL: "https://youtube.com/watch?v=a", Kind: KindStream, Clip: c})
		if !errors.Is(err, ErrBadClip) {
			t.Errorf("clip %v: error %v, want ErrBadClip", c, err)
		}
	}
}
