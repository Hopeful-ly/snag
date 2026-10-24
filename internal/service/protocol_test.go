package service_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/httpdl/httpdltest"
	"github.com/Roham1299/snag/internal/ipc"
	"github.com/Roham1299/snag/internal/pagedl"
	"github.com/Roham1299/snag/internal/playlist"
	"github.com/Roham1299/snag/internal/service"
	"github.com/Roham1299/snag/internal/tools"
)

func TestPing(t *testing.T) {
	c := connect(t, startApp(t, ""))
	reply := c.ask(t, "ping")
	if !reply.ok() || reply["version"] != 1.0 || reply["ytdlp"] != false {
		t.Errorf("ping = %v", reply)
	}
}

func TestAddDownloadsAndReportsChanges(t *testing.T) {
	srv := httpdltest.NewServer(t)
	data := make([]byte, 3<<20)
	_, _ = rand.Read(data)
	url := srv.Add("/files/Nivalis-Nights.zip", data, httpdltest.ContentType("application/zip"))
	app := startApp(t, `{"ask": false}`)
	c := connect(t, app)

	if reply := c.ask(t, "subscribe"); !reply.ok() {
		t.Fatalf("subscribe = %v", reply)
	}
	reply := c.ask(t, "add", "url", url, "kind", "http", "page", "https://example.com/", "cookies",
		[]any{map[string]any{"name": "s", "value": "1", "domain": "127.0.0.1"}})
	d := reply.obj("download")
	if !reply.ok() || d.str("kind") != "http" || reply["duplicate"] != false || d.str("name") != "Nivalis-Nights.zip" {
		t.Fatalf("add = %v", reply)
	}

	done := c.waitEvent(t, func(m message) bool { return m.str("type") == "update" && m.obj("download").str("status") == "done" })
	got := done.obj("download")
	if got.str("path") != filepath.Join(app.paths.Downloads, "Nivalis-Nights.zip") || got["size"] != float64(len(data)) {
		t.Errorf("finished download = %v", got)
	}
	for _, key := range []string{"eta", "connections", "created", "finished", "resumable", "estimated"} {
		if _, ok := got[key]; !ok {
			t.Errorf("a download on the wire lacks %q", key)
		}
	}
	if file, _ := os.ReadFile(got.str("path")); len(file) != len(data) {
		t.Errorf("file has %d bytes, want %d", len(file), len(data))
	}
}

func TestAddOptions(t *testing.T) {
	app := startApp(t, `{"ask": true}`)
	c := connect(t, app)

	paused := c.ask(t, "add", "url", "https://example.com/a.zip", "start", false).obj("download")
	if paused.str("status") != "paused" {
		t.Errorf("start:false gave %q", paused.str("status"))
	}
	again := c.ask(t, "add", "url", "https://example.com/a.zip", "start", false)
	if again["duplicate"] != true || again.obj("download").str("id") != paused.str("id") {
		t.Errorf("adding it again = %v", again)
	}
	pending := c.ask(t, "add", "url", "https://example.com/b.zip", "start", false, "ask", true)
	if s := pending.obj("download").str("status"); s != "paused" {
		t.Errorf("start:false wins over asking: %q", s)
	}
	asked := c.ask(t, "add", "url", "https://example.com/c.zip").obj("download")
	if asked.str("status") != "pending" {
		t.Errorf("with ask on, status %q, want pending", asked.str("status"))
	}
	stream := c.ask(t, "add", "url", "https://example.com/v.m3u8", "kind", "ytdlp", "start", false).obj("download")
	if stream.str("kind") != "ytdlp" {
		t.Errorf("a stream's kind on the wire = %q, want ytdlp", stream.str("kind"))
	}

	for _, bad := range []message{
		{"url": "file:///etc/passwd"},
		{"url": "https://example.com/x", "kind": "torrent"},
		{"url": "https://example.com/x", "headers": "not an object"},
	} {
		var fields []any
		for k, v := range bad {
			fields = append(fields, k, v)
		}
		if reply := c.ask(t, "add", fields...); reply.ok() || reply.str("error") == "" {
			t.Errorf("add %v = %v, want an error", bad, reply)
		}
	}
}

func TestHaveFindsWhatTheListAlreadyHas(t *testing.T) {
	app := startApp(t, `{"ask": false}`)
	c := connect(t, app)
	d := c.ask(t, "add",
		"url", "https://youtube.com/watch?v=a", "kind", "ytdlp", "start", false,
		"page", "https://youtube.com/watch?v=a", "title", "Severance S02E03 - Cold Harbor",
		"label", "1080p", "format", "137+140",
	).obj("download")

	reply := c.ask(t, "have",
		"page", "https://youtube.com/watch?v=a", "title", "Severance S02E03 - Cold Harbor",
		"items", []any{
			map[string]any{"url": "https://youtube.com/watch?v=a", "label": "1080p", "format": "137+140"},
			map[string]any{"url": "https://youtube.com/watch?v=a", "label": "2160p", "format": "401+140"},
			map[string]any{"url": "https://youtube.com/watch?v=other", "label": "1080p"},
		},
	)
	if !reply.ok() {
		t.Fatalf("have = %v", reply)
	}
	list, _ := reply["have"].([]any)
	if len(list) != 3 {
		t.Fatalf("have entries = %v", reply["have"])
	}
	got, _ := list[0].(map[string]any)
	if got == nil || got["id"] != d["id"] {
		t.Errorf("same address: %v, want %s", list[0], d.str("id"))
	}
	if list[1] != nil {
		t.Errorf("a different quality: %v", list[1])
	}
	episode, _ := list[2].(map[string]any)
	if episode == nil || episode["id"] != d["id"] {
		t.Errorf("same episode: %v", list[2])
	}
}

func TestConfirmRemembersTheFolder(t *testing.T) {
	app := startApp(t, `{"ask": true}`)
	c := connect(t, app)
	c.ask(t, "subscribe")
	d := c.ask(t, "add", "url", "https://example.com/a.zip").obj("download")

	folder := filepath.Join(t.TempDir(), "picked folder")
	if reply := c.ask(t, "confirm", "id", d.str("id"), "directory", folder, "filename", "My name.zip", "start", false); !reply.ok() {
		t.Fatalf("confirm = %v", reply)
	}
	event := c.waitEvent(t, func(m message) bool { return m.str("type") == "config" })
	recent, _ := event.obj("config")["recent_dirs"].([]any)
	if len(recent) == 0 || recent[0] != folder {
		t.Errorf("recent_dirs = %v, want %q first", recent, folder)
	}
	if reply := c.ask(t, "confirm", "id", d.str("id"), "directory", folder); reply.ok() {
		t.Error("confirming twice was accepted")
	}
}

func TestConfig(t *testing.T) {
	app := startApp(t, "")
	c := connect(t, app)
	config := c.ask(t, "config").obj("config")
	if config["connections"] != 8.0 || config.str("download_dir") != app.paths.Downloads {
		t.Errorf("config = %v", config)
	}
	changed := c.ask(t, "config", "set", map[string]any{"connections": 4, "notify": false}).obj("config")
	if changed["connections"] != 4.0 || changed["notify"] != false {
		t.Errorf("after set: %v", changed)
	}
	if saved, _ := os.ReadFile(app.paths.Config); !strings.Contains(string(saved), `"connections": 4`) {
		t.Errorf("settings file: %s", saved)
	}
	if reply := c.ask(t, "config", "set", map[string]any{"colour": "red"}); reply.ok() {
		t.Error("an unknown setting was accepted")
	}
	if reply := c.ask(t, "config", "set", map[string]any{"connections": "many"}); reply.ok() {
		t.Error("a setting of the wrong type was accepted")
	}
}

func TestControls(t *testing.T) {
	app := startApp(t, `{"ask": false}`)
	c := connect(t, app)
	d := c.ask(t, "add", "url", "https://example.com/a.zip", "start", false).obj("download")
	id := d.str("id")

	if reply := c.ask(t, "resume_all"); !reply.ok() {
		t.Errorf("resume_all = %v", reply)
	}
	if reply := c.ask(t, "pause", "id", id); !reply.ok() {
		t.Errorf("pause = %v", reply)
	}
	if reply := c.ask(t, "open", "id", id); reply.ok() || !strings.Contains(reply.str("error"), "not finished") {
		t.Errorf("opening an unfinished download = %v", reply)
	}
	if reply := c.ask(t, "remove", "id", id); !reply.ok() {
		t.Errorf("remove = %v", reply)
	}
	if reply := c.ask(t, "pause", "id", "nope"); reply.ok() {
		t.Error("pausing an unknown download was accepted")
	}
	if reply := c.ask(t, "show", "id", id); !reply.ok() {
		t.Errorf("show = %v", reply)
	}
	if shown, n := app.window.last(); n != 1 || shown != id {
		t.Errorf("window showed %q (%d times)", shown, n)
	}
}

func TestKindAndProbe(t *testing.T) {
	c := connect(t, startApp(t, ""))
	for address, want := range map[string]string{
		"https://cdn.example.com/live/index.m3u8": "ytdlp",
		"https://example.com/files/setup.exe":     "http",
	} {
		if got := c.ask(t, "kind", "url", address).str("kind"); got != want {
			t.Errorf("kind of %s = %q, want %q", address, got, want)
		}
	}
	probe := c.ask(t, "probe", "url", "https://example.com/watch")
	if !probe.ok() || probe.obj("probe")["supported"] != false || probe.obj("probe").str("reason") != "yt-dlp is missing" {
		t.Errorf("probe without yt-dlp = %v", probe)
	}
	if reply := c.ask(t, "probe", "url", "javascript:alert(1)"); reply.ok() {
		t.Error("probing a non-web address was accepted")
	}
}

func TestBadRequests(t *testing.T) {
	app := startApp(t, "")
	c := connect(t, app)
	if reply := c.ask(t, "quit"); reply.ok() || !strings.Contains(reply.str("error"), "unknown request") {
		t.Errorf("unknown request = %v", reply)
	}

	raw, err := ipc.Dial(app.paths.Socket, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	conn := ipc.NewConn(raw)
	defer conn.Close()
	_ = conn.SendRaw([]byte("{not json"))
	line, err := conn.Receive()
	var reply message
	if err != nil || json.Unmarshal(line, &reply) != nil || reply.ok() {
		t.Errorf("reply to broken JSON = %s, %v", line, err)
	}
}

func TestOneAppPerSocket(t *testing.T) {
	first := startApp(t, "")
	_, err := service.Start(service.Options{Paths: first.paths, Tools: tools.Paths{}})
	if !errors.Is(err, ipc.ErrInUse) {
		t.Errorf("second app: %v, want ErrInUse", err)
	}
	if downloads := first.Engine.List(); len(downloads) != 0 {
		t.Errorf("the second app touched the first's list: %v", downloads)
	}
}

func TestBrowserContactIsNoticedAndRemembered(t *testing.T) {
	app := startApp(t, "")
	contacts := make(chan struct{}, 10)
	app.OnBrowserContact(func() { contacts <- struct{}{} })
	if !app.LastContact().IsZero() || app.Config().BrowserConnected {
		t.Fatal("contact before any")
	}

	c := connect(t, app)
	c.ask(t, "ping")
	c.ask(t, "list")
	<-contacts
	if app.LastContact().IsZero() || !app.Config().BrowserConnected {
		t.Error("contact not recorded")
	}
	select {
	case <-contacts:
		t.Error("told about contact more than once in a run")
	default:
	}
}

// twoVideos lists any page with "list=" as two videos.
type twoVideos struct{}

func (twoVideos) List(_ context.Context, req playlist.Request) (playlist.Playlist, error) {
	if !strings.Contains(req.URL, "list=") {
		return playlist.Playlist{}, playlist.ErrNotAList
	}
	return playlist.Playlist{Title: "Mix", Page: req.URL, Current: 1, Entries: []playlist.Entry{
		{URL: "https://v.example/1"}, {URL: "https://v.example/2"},
	}}, nil
}

func TestPlaylistRequest(t *testing.T) {
	app := startApp(t, "")
	app.Playlists = twoVideos{}
	c := connect(t, app)

	reply := c.ask(t, "playlist", "url", "https://v.example/watch?v=2&list=mix")
	found := reply.obj("playlist")
	if !reply.ok() || found.str("title") != "Mix" || found["count"] != 2.0 || found.str("id") == "" {
		t.Fatalf("playlist = %v", reply)
	}
	if shown, _ := app.window.last(); shown != "playlist "+found.str("id") {
		t.Errorf("the window showed %q", shown)
	}
	if kept, err := app.Playlist(found.str("id")); err != nil || len(kept.Entries) != 2 {
		t.Errorf("kept %+v, %v", kept, err)
	}

	reply = c.ask(t, "playlist", "url", "https://v.example/watch?v=2")
	if !reply.ok() || reply["playlist"] != nil {
		t.Errorf("a page of one video = %v", reply)
	}
	if _, shown := app.window.last(); shown != 1 {
		t.Errorf("the window showed %d times", shown)
	}
	if reply := c.ask(t, "playlist", "url", "file:///etc"); reply.ok() {
		t.Error("listing a non-web address was accepted")
	}
}

func TestResolve(t *testing.T) {
	app := startApp(t, "")
	page := "https://site.example/tv/1/1/2?play=true"
	if _, err := app.Resolve(context.Background(), pagedl.Request{Page: page}); !errors.Is(err, pagedl.ErrNoBrowser) {
		t.Fatalf("with no browser: %v", err)
	}

	c := connect(t, app)
	c.ask(t, "subscribe")
	type answer struct {
		src pagedl.Source
		err error
	}
	answers := make(chan answer, 1)
	go func() {
		src, err := app.Resolve(context.Background(), pagedl.Request{Page: page})
		answers <- answer{src, err}
	}()
	asked := c.waitEvent(t, func(m message) bool { return m.str("type") == "resolve" })
	if asked.str("page") != page || asked.str("token") == "" {
		t.Fatalf("asked %v", asked)
	}
	stream := "https://cdn.example/v/index.m3u8"
	reply := c.ask(t, "resolved", "token", asked.str("token"), "url", stream,
		"headers", map[string]string{"Referer": "https://site.example/"})
	if !reply.ok() {
		t.Fatalf("resolved = %v", reply)
	}
	got := <-answers
	if got.err != nil || got.src.URL != stream || got.src.Headers["Referer"] != "https://site.example/" {
		t.Errorf("resolved to %+v, %v", got.src, got.err)
	}
	if reply := c.ask(t, "resolved", "token", asked.str("token"), "url", stream); reply.ok() {
		t.Error("an answer to nothing asked was taken")
	}

	go func() {
		_, err := app.Resolve(context.Background(), pagedl.Request{Page: page})
		answers <- answer{err: err}
	}()
	asked = c.waitEvent(t, func(m message) bool { return m.str("type") == "resolve" })
	c.ask(t, "resolve_failed", "token", asked.str("token"), "error", "no video played")
	if got := <-answers; got.err == nil || got.err.Error() != "couldn't find the video on site.example: no video played" {
		t.Errorf("a failed resolve: %v", got.err)
	}
}
