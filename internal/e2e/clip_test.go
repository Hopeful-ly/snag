package e2e

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/nativehost"
	"github.com/Roham1299/snag/internal/tools"
)

func TestClipFromThePlayer(t *testing.T) {
	browser := findBrowser(t)
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil || tools.Find("").YtDLP == "" {
		t.Skip("needs ffmpeg and yt-dlp")
	}
	media := t.TempDir()
	if out, err := exec.Command(ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=duration=60:size=640x360:rate=25",
		"-f", "lavfi", "-i", "sine=duration=60", "-c:v", "libx264", "-g", "50", "-c:a", "aac", "-movflags", "+faststart",
		filepath.Join(media, "film.mp4")).CombinedOutput(); err != nil {
		t.Fatalf("making the video: %v\n%s", err, out)
	}
	files := httptest.NewServer(http.FileServer(http.Dir(media)))
	t.Cleanup(files.Close)
	pages := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<!doctype html><title>Holiday</title><body style="margin:0;background:#111">`+
			`<video src=%q controls muted preload=auto width=780 height=440></video>`, files.URL+"/film.mp4")
	}))
	t.Cleanup(pages.Close)

	root := t.TempDir()
	host := buildHost(t, root)
	app, socket := startApp(t, root)
	profile := filepath.Join(root, "profile")
	if _, err := nativehost.Register(host, []nativehost.Browser{{Name: "test", Family: nativehost.Chromium,
		ManifestDir: filepath.Join(profile, "NativeMessagingHosts")}}); err != nil {
		t.Fatal(err)
	}
	port := launch(t, browser, profile, buildExtension(t), socket)
	tab := attach(t, port, "about:blank")
	tab.call(t, "Runtime.enable", nil)
	tab.call(t, "Page.navigate", map[string]any{"url": pages.URL + "/watch"})
	tab.waitForExtension(t)
	tab.evaluate(t, 0, `new Promise((ok) => { const v = document.querySelector("video");
		v.readyState >= 1 ? ok(true) : v.addEventListener("loadedmetadata", () => ok(true)); })`, nil)
	checkDownloadButton(t, tab, "film.mp4")

	tab.click(t, tab.waitForNode(t, "the Clip switch", func(n domNode) bool { return n.hasClass("chip") && n.text() == "Clip" }))
	tab.waitForNode(t, "the trim bar", func(n domNode) bool { return n.hasClass("trim", "on") })
	tab.screenshot(t, "clip-open.png")

	doc := tab.document(t)
	rail := doc.find(func(n domNode) bool { return n.hasClass("rail") })
	from := doc.find(func(n domNode) bool { return n.hasClass("handle", "from") })
	left, _, width := tab.span(t, rail.NodeID)
	x, y := tab.centerOf(t, from.NodeID)
	tab.drag(t, x, y, left+width*0.1, y)
	tab.evaluate(t, 0, `document.querySelector("video").currentTime = 15.5`, nil)
	tab.click(t, tab.waitForNode(t, "the end button", func(n domNode) bool {
		return n.hasClass("tbtn") && n.attribute("title") == "End the clip where the video is"
	}))
	times := tab.waitForNode(t, "the clip's times", func(n domNode) bool {
		return n.hasClass("trim-times") && strings.HasSuffix(n.text(), "0:15.5")
	}).text()
	start, err := strconv.ParseFloat(strings.TrimPrefix(strings.Fields(times)[0], "0:"), 64)
	if err != nil || math.Abs(start-6) > 0.6 {
		t.Fatalf("dragging the start to a tenth of a minute made the clip %q", times)
	}
	tab.screenshot(t, "clip-set.png")

	tab.click(t, tab.waitForNode(t, "the file in the card", func(n domNode) bool {
		return n.hasClass("file") && n.find(func(c domNode) bool { return c.hasClass("fname") }) != nil
	}))
	d := waitAdded(t, app.Engine.List)
	done := waitStatus(t, app, d.ID, engine.StatusDone, engine.StatusError)
	if done.Status != engine.StatusDone {
		t.Fatalf("the clip ended %q: %s", done.Status, done.Error)
	}
	if done.Clip.To != 15.5 || math.Abs(done.Clip.From-start) > 0.06 {
		t.Errorf("clip %+v, want %v-15.5", done.Clip, start)
	}
	if want := fmt.Sprintf("film (0.%02d-0.15).mp4", int(start)); filepath.Base(done.Path) != want {
		t.Errorf("saved as %q, want %q", filepath.Base(done.Path), want)
	}
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", done.Path).Output()
	length, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || math.Abs(length-done.Clip.Length()) > 0.1 {
		t.Errorf("the file is %.2fs long, want %.2fs (%v)", length, done.Clip.Length(), err)
	}
}

func waitAdded(t *testing.T, list func() []engine.Download) engine.Download {
	t.Helper()
	for range 100 {
		if l := list(); len(l) > 0 {
			return l[0]
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("nothing was added")
	return engine.Download{}
}

func (c *cdp) click(t *testing.T, n domNode) {
	t.Helper()
	x, y := c.centerOf(t, n.NodeID)
	for _, kind := range []string{"mousePressed", "mouseReleased"} {
		c.call(t, "Input.dispatchMouseEvent", map[string]any{"type": kind, "x": x, "y": y, "button": "left", "clickCount": 1})
	}
}

func (c *cdp) drag(t *testing.T, x0, y0, x1, y1 float64) {
	t.Helper()
	c.call(t, "Input.dispatchMouseEvent", map[string]any{"type": "mousePressed", "x": x0, "y": y0, "button": "left", "clickCount": 1})
	for i := 1; i <= 10; i++ {
		x, y := x0+(x1-x0)*float64(i)/10, y0+(y1-y0)*float64(i)/10
		c.call(t, "Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": x, "y": y, "button": "left", "buttons": 1})
	}
	c.call(t, "Input.dispatchMouseEvent", map[string]any{"type": "mouseReleased", "x": x1, "y": y1, "button": "left", "clickCount": 1})
}

func (c *cdp) span(t *testing.T, nodeID int) (left, top, width float64) {
	t.Helper()
	x, y := c.centerOf(t, nodeID)
	var box struct {
		Model struct {
			Width float64 `json:"width"`
		} `json:"model"`
	}
	raw := c.call(t, "DOM.getBoxModel", map[string]any{"nodeId": nodeID})
	if err := json.Unmarshal(raw, &box); err != nil {
		t.Fatal(err)
	}
	return x - box.Model.Width/2, y, box.Model.Width
}

// screenshot saves the page to $SNAG_SHOTS, when set, for looking at.
func (c *cdp) screenshot(t *testing.T, name string) {
	t.Helper()
	dir := os.Getenv("SNAG_SHOTS")
	if dir == "" {
		return
	}
	var shot struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(c.call(t, "Page.captureScreenshot", map[string]any{"format": "png"}), &shot); err != nil {
		t.Fatal(err)
	}
	data, _ := base64.StdEncoding.DecodeString(shot.Data)
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
