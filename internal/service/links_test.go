package service_test

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/httpdl/httpdltest"
	"github.com/Roham1299/snag/internal/links"
)

func TestLinksFromAPage(t *testing.T) {
	srv := httpdltest.NewServer(t)
	data := make([]byte, 300_000)
	_, _ = rand.Read(data)
	zip := srv.Add("/pub/tool.zip", data, httpdltest.ContentType("application/zip"))
	named := srv.Add("/pub/get", data[:1000], httpdltest.Filename("Manual.pdf"), httpdltest.ContentType("application/pdf"))
	nested := srv.Add("/pub/iso/distro.iso", data[:5000], httpdltest.ContentType("application/octet-stream"))

	app := startApp(t, `{"ask": true}`)
	c := connect(t, app)
	reply := c.ask(t, "links", "page", srv.URL+"/pub/", "title", "Pub", "links", []any{
		map[string]any{"url": "tool.zip", "text": "The tool"},
		map[string]any{"url": named, "text": "Manual"},
		map[string]any{"url": "iso/distro.iso"},
		map[string]any{"url": "#top"},
		map[string]any{"url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ"},
	})
	found := reply.obj("links")
	if !reply.ok() || found["count"] != 4.0 {
		t.Fatalf("links = %v", reply)
	}
	id := found.str("id")
	if shown, _ := app.window.last(); shown != "links "+id {
		t.Errorf("window shown: %q", shown)
	}

	set, err := app.Links(id)
	if err != nil {
		t.Fatal(err)
	}
	groups := []links.Group{set.Links[0].Group, set.Links[1].Group, set.Links[2].Group, set.Links[3].Group}
	if groups[0] != links.Archives || groups[1] != links.Pages || groups[2] != links.Archives || groups[3] != links.VideoPages {
		t.Errorf("groups = %v", groups)
	}

	looked, err := app.LookAtLinks(context.Background(), id, []int{0, 1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	// The link that looked like a page names a PDF; video pages aren't asked.
	if looked[0].Size != int64(len(data)) || looked[1].Server != "Manual.pdf" || looked[1].Group != links.Documents ||
		looked[3].Looked {
		t.Errorf("looked = %+v", looked)
	}

	dir := filepath.Join(app.paths.Downloads, "Pub")
	added, err := app.DownloadLinks(id, links.Choice{Entries: []int{0, 1, 2}, Directory: dir, KeepFolders: true})
	if err != nil || added.Added != 3 {
		t.Fatalf("DownloadLinks = %+v, %v", added, err)
	}
	want := map[string]int{
		filepath.Join(dir, "tool.zip"):          len(data),
		filepath.Join(dir, "Manual.pdf"):        1000,
		filepath.Join(dir, "iso", "distro.iso"): 5000,
	}
	for _, d := range waitAllDone(t, app, 3) {
		if want[d.Path] == 0 {
			t.Errorf("unexpected download %s", d.Path)
			continue
		}
		if info, err := os.Stat(d.Path); err != nil || info.Size() != int64(want[d.Path]) {
			t.Errorf("%s: %v, %v", d.Path, info, err)
		}
	}
	_ = zip
	_ = nested

	have, err := app.LinksHave(id)
	if err != nil || !have[0] || !have[1] || !have[2] || have[3] {
		t.Errorf("have = %v, %v", have, err)
	}
	if again, _ := app.DownloadLinks(id, links.Choice{Entries: []int{0}, Directory: dir}); again.Added != 1 {
		// A finished download isn't "unfinished already": it is downloaded again, as a new file.
		t.Errorf("again = %+v", again)
	}
}

func TestPastedLinks(t *testing.T) {
	app := startApp(t, "")
	set, err := app.LinksFromText("get https://example.org/a.zip and\nhttps://example.org/b.zip, thanks")
	if err != nil || len(set.Links) != 2 || set.Source != links.FromPasted {
		t.Fatalf("LinksFromText = %+v, %v", set, err)
	}
	if _, err := app.LinksFromText("no links here"); err == nil {
		t.Error("kept a set of nothing")
	}
	if _, err := app.DownloadLinks(set.ID, links.Choice{Entries: []int{0}, Directory: t.TempDir(), Queue: "nope"}); err == nil ||
		!strings.Contains(err.Error(), "queue") {
		t.Errorf("a queue that isn't there: %v", err)
	}
}

func waitAllDone(t *testing.T, app *testApp, n int) []engine.Download {
	t.Helper()
	for range 600 {
		list := app.Engine.List()
		done := 0
		for _, d := range list {
			if d.Status == engine.StatusDone {
				done++
			}
			if d.Status == engine.StatusError {
				t.Fatalf("%s failed: %s", d.URL, d.Error)
			}
		}
		if done >= n {
			return list
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("not all %d finished: %+v", n, app.Engine.List())
	return nil
}
