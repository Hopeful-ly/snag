package engine

import (
	"os"
	"testing"
)

func TestHaveFindsTheSameAddressAndQuality(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.add(t, AddRequest{
		URL: "https://youtube.com/watch?v=a", Title: "Severance S02E03 - Cold Harbor",
		Label: "1080p", Format: "137+140", Kind: KindStream, Paused: true,
	})
	got, ok := te.Have(HaveQuery{URL: d.URL, Format: "137+140"})
	if !ok || got.ID != d.ID {
		t.Fatalf("same address: %+v, %v", got, ok)
	}
	if _, ok := te.Have(HaveQuery{URL: d.URL, Format: "401+140"}); ok {
		t.Fatal("a different quality of the same page URL")
	}
}

func TestHaveFindsTheSameEpisodeAtThatQuality(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.add(t, AddRequest{
		URL: "https://youtube.com/watch?v=old", Page: "https://youtube.com/watch?v=old",
		Title: "Severance S02E03 - Cold Harbor", Label: "1080p", Format: "137+140",
		Kind: KindStream, Paused: true,
	})
	got, ok := te.Have(HaveQuery{
		URL: "https://youtube.com/watch?v=new", Title: "Severance S02E03 - Cold Harbor.mp4", Label: "1080p",
	})
	if !ok || got.ID != d.ID {
		t.Fatalf("same episode: %+v, %v", got, ok)
	}
	if _, ok := te.Have(HaveQuery{Title: "Severance S02E03 - Cold Harbor", Label: "2160p"}); ok {
		t.Fatal("the same episode at another quality")
	}
	if _, ok := te.Have(HaveQuery{Title: "The Bear S02E03", Label: "1080p"}); ok {
		t.Fatal("another show's episode")
	}
}

func TestHaveFindsTheEpisodeOnTheFileNameWhenThePageTitleIsGeneric(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.add(t, AddRequest{
		URL: "https://youtube.com/watch?v=old", Title: "Watch on YouTube",
		Filename: "Severance S02E03 - Cold Harbor", Label: "1080p", Format: "137+140",
		Kind: KindStream, Paused: true,
	})
	got, ok := te.Have(HaveQuery{
		Title: "YouTube", Filename: "Severance S02E03 - Cold Harbor.mp4", Label: "1080p",
	})
	if !ok || got.ID != d.ID {
		t.Fatalf("episode on the file name: %+v, %v", got, ok)
	}
}

func TestHavePrefersTheOneStillRunning(t *testing.T) {
	te := newTestEngine(t, Settings{})
	older := te.add(t, AddRequest{
		URL: "https://youtube.com/watch?v=old", Title: "Severance S02E03", Label: "1080p",
		Format: "137+140", Kind: KindStream, Paused: true,
	})
	newer := te.add(t, AddRequest{
		URL: "https://youtube.com/watch?v=new", Title: "Severance S02E03", Label: "1080p",
		Format: "22", Kind: KindStream, Paused: false,
	})
	got, ok := te.Have(HaveQuery{Title: "Severance S02E03", Label: "1080p"})
	if !ok || got.ID != newer.ID {
		t.Fatalf("prefer running %s over paused %s: %+v, %v", newer.ID, older.ID, got, ok)
	}
}

func TestHaveIgnoresAFinishedDownloadWhoseFileIsGone(t *testing.T) {
	te := newTestEngine(t, Settings{})
	d := te.add(t, AddRequest{URL: "https://example.com/a.bin", Paused: false})
	te.waitFor(t, d.ID, StatusDownloading)
	te.runner.finish(d.URL, nil)
	d = te.waitFor(t, d.ID, StatusDone)
	if _, ok := te.Have(HaveQuery{URL: d.URL}); !ok {
		t.Fatal("a finished file should count")
	}
	if err := os.Remove(d.Path); err != nil {
		t.Fatal(err)
	}
	if _, ok := te.Have(HaveQuery{URL: d.URL}); ok {
		t.Fatal("gone from disk, still offered as have")
	}
}
