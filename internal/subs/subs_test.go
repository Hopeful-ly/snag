package subs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVttToSRT(t *testing.T) {
	in := []byte("WEBVTT\n\n00:00:01.000 --> 00:00:03.000 line:90%\nHello\n\n00:01:02.500 --> 00:01:04.000\nThere\n")
	got, err := vttToSRT(in)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.Contains(s, "1\n00:00:01,000 --> 00:00:03,000\nHello") {
		t.Errorf("first cue:\n%s", s)
	}
	if !strings.Contains(s, "2\n00:01:02,500 --> 00:01:04,000\nThere") {
		t.Errorf("second cue:\n%s", s)
	}
}

func TestVttToSRTAddsHours(t *testing.T) {
	in := []byte("WEBVTT\n\n01:02.000 --> 01:04.000\nHi\n")
	got, err := vttToSRT(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "00:01:02,000 --> 00:01:04,000") {
		t.Errorf("got %s", got)
	}
}

func TestSidecarPath(t *testing.T) {
	if got := sidecarPath("/dl/Movie.mp4", "en"); got != "/dl/Movie.en.srt" {
		t.Errorf("en: %s", got)
	}
	if got := sidecarPath("/dl/Movie.mp4", "und"); got != "/dl/Movie.srt" {
		t.Errorf("und: %s", got)
	}
}

func TestAttachWritesCuesAlreadyInHand(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "Movie.mp4")
	if err := os.WriteFile(video, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := "1\n00:00:01,000 --> 00:00:02,000\nHi\n"
	err := Attach(context.Background(), Request{
		Text:    body,
		Video:   video,
		Lang:    "en",
		Sidecar: true,
		WorkDir: filepath.Join(dir, "work"),
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "Movie.en.srt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Errorf("sidecar = %q", got)
	}
}

func TestAttachWritesSidecar(t *testing.T) {
	body := "1\n00:00:01,000 --> 00:00:02,000\nHi\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-subrip")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	dir := t.TempDir()
	video := filepath.Join(dir, "Movie.mp4")
	if err := os.WriteFile(video, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(dir, "work")
	err := Attach(context.Background(), Request{
		URL:     srv.URL + "/en.srt",
		Video:   video,
		Lang:    "en",
		Sidecar: true,
		WorkDir: work,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "Movie.en.srt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Errorf("sidecar = %q", got)
	}
}
