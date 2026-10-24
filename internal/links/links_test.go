package links

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/engine"
)

func TestClassify(t *testing.T) {
	for address, want := range map[string]Group{
		"https://example.org/tool-1.0.tar.gz":         Archives,
		"https://example.org/files/distro.iso":        Archives,
		"https://example.org/Setup.EXE":               Programs,
		"https://example.org/app.AppImage?dl=1":       Programs,
		"https://example.org/v/film.mkv":              Videos,
		"https://example.org/a/song.flac":             Audio,
		"https://example.org/report.pdf":              Documents,
		"https://example.org/photo.jpeg":              Images,
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ": VideoPages,
		"https://youtu.be/dQw4w9WgXcQ":                VideoPages,
		"https://vimeo.com/123456":                    VideoPages,
		"https://example.org/about":                   Pages,
		"https://example.org/":                        Pages,
		"https://example.org/index.php?page=2":        Pages,
		"https://example.org/data.parquet":            Other,
	} {
		if got := Classify(address, ""); got != want {
			t.Errorf("Classify(%s) = %s, want %s", address, got, want)
		}
	}
	// The name a page gives wins over a token in the address.
	if got := Classify("https://cdn.example.org/get?id=81723", "Manual.pdf"); got != Documents {
		t.Errorf("named = %s", got)
	}
}

func TestNewTidiesTheLinks(t *testing.T) {
	set, err := New("https://example.org/pub/linux/index.html", "  Linux ", FromPage, []Found{
		{URL: "iso/24.04/distro.iso", Text: "Distro\n  24.04"},
		{URL: "https://example.org/pub/linux/iso/24.04/distro.iso#top"}, // the same, again
		{URL: "#top"}, // the page itself
		{URL: "index.html"},
		{URL: "mailto:me@example.org"},
		{URL: "javascript:void(0)"},
		{URL: "/get?id=2", Name: "Manual (EN).pdf", Text: "Manual"},
		{URL: "https://mirror.example.net/pub/linux/other.zip"},
		{URL: "../up.zip"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []Link{
		{URL: "https://example.org/pub/linux/iso/24.04/distro.iso", Text: "Distro 24.04", Name: "distro.iso", Group: Archives, Dir: "iso/24.04"},
		{URL: "https://example.org/get?id=2", Text: "Manual", Name: "Manual (EN).pdf", Group: Documents},
		{URL: "https://mirror.example.net/pub/linux/other.zip", Name: "other.zip", Group: Archives},
		{URL: "https://example.org/pub/up.zip", Name: "up.zip", Group: Archives},
	}
	if set.Title != "Linux" || len(set.Links) != len(want) {
		t.Fatalf("set = %+v", set)
	}
	for i, l := range set.Links {
		if l != want[i] {
			t.Errorf("link %d = %+v\n want %+v", i, l, want[i])
		}
	}

	if _, err := New("https://example.org/", "", FromPage, []Found{{URL: "#"}, {URL: "ftp://x/y"}}); !errors.Is(err, ErrNoLinks) {
		t.Errorf("a page without links: %v", err)
	}
}

func TestInText(t *testing.T) {
	text := "Here: https://example.org/a.zip, and (https://example.org/b.iso).\nhttps://example.org/c?x=1&y=2\nnot a link: example.org/d"
	var got []string
	for _, f := range InText(text) {
		got = append(got, f.URL)
	}
	want := []string{"https://example.org/a.zip", "https://example.org/b.iso", "https://example.org/c?x=1&y=2"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("InText = %v, want %v", got, want)
	}
}

func TestPlan(t *testing.T) {
	set, err := New("https://example.org/pub/", "Pub", FromPage, []Found{
		{URL: "a.zip"},
		{URL: "sub/b.iso"},
		{URL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ", Text: "A video"},
	})
	if err != nil {
		t.Fatal(err)
	}
	set.Headers = map[string]string{"Referer": "https://example.org/pub/"}
	set.Links[0].Server, set.Links[0].Size = "a-1.0.zip", 1234

	reqs, err := Plan(set, Choice{Entries: []int{2, 0, 1, 0}, Directory: "/dl", Queue: "q1", KeepFolders: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 3 {
		t.Fatalf("%d downloads, want 3", len(reqs))
	}
	video, a, b := reqs[0], reqs[1], reqs[2]
	if video.Kind != engine.KindStream || video.Format == "" || video.Page != video.URL || video.Filename != "" {
		t.Errorf("the video = %+v", video)
	}
	if a.Kind != engine.KindHTTP || a.Filename != "a-1.0.zip" || a.Size != 1234 || a.Directory != "/dl" ||
		a.Queue != "q1" || !a.Confirmed || a.Headers["Referer"] == "" || a.Page != set.Page {
		t.Errorf("a = %+v", a)
	}
	if b.Directory != filepath.Join("/dl", "sub") || b.Filename != "b.iso" {
		t.Errorf("b = %+v", b)
	}

	if _, err := Plan(set, Choice{Directory: "/dl"}); err == nil {
		t.Error("planned nothing")
	}
	if _, err := Plan(set, Choice{Entries: []int{9}, Directory: "/dl"}); err == nil {
		t.Error("planned a link that isn't there")
	}
	if _, err := Plan(set, Choice{Entries: []int{0}}); err == nil {
		t.Error("planned without a folder")
	}
}

func TestStore(t *testing.T) {
	now := time.Now()
	st := &Store{now: func() time.Time { return now }}
	s := st.Put(Set{Title: "x", Links: []Link{{URL: "https://e.org/a"}}})
	if s.ID == "" {
		t.Fatal("no id")
	}
	st.Learned(s.ID, 0, 99, "a.bin", false)
	st.Learned(s.ID, 5, 1, "nope", false) // no such link
	got, ok := st.Get(s.ID)
	if !ok || got.Links[0].Size != 99 || got.Links[0].Server != "a.bin" || !got.Links[0].Looked {
		t.Fatalf("Get = %+v, %v", got, ok)
	}
	now = now.Add(2 * time.Hour)
	if _, ok := st.Get(s.ID); ok {
		t.Fatal("kept too long")
	}
}

func TestLearningRegroups(t *testing.T) {
	st := &Store{}
	s := st.Put(Set{Links: []Link{
		{URL: "https://e.org/get?id=1", Group: Pages},
		{URL: "https://e.org/get?id=2", Group: Pages},
		{URL: "https://e.org/a.zip", Group: Archives},
		{URL: "https://e.org/b", Group: Pages},
	}})
	st.Learned(s.ID, 0, 10, "Manual.pdf", false)
	st.Learned(s.ID, 1, 10, "", false)
	st.Learned(s.ID, 2, 0, "", true) // a page saying the file is gone
	st.Learned(s.ID, 3, 0, "", true)
	got, _ := st.Get(s.ID)
	for i, want := range []Group{Documents, Other, Pages, Pages} {
		if got.Links[i].Group != want {
			t.Errorf("link %d is %s, want %s", i, got.Links[i].Group, want)
		}
	}
}

func TestAVideoPageIsNamedByItsWords(t *testing.T) {
	set, err := New("https://example.org/", "", FromPage, []Found{{URL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ", Text: "The launch video"}})
	if err != nil || set.Links[0].Name != "The launch video" {
		t.Fatalf("set = %+v, %v", set, err)
	}
}
