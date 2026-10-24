// Package e2e runs the unmodified extension against the app in a real headless browser.
package e2e

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Roham1299/snag/extension"
	"github.com/Roham1299/snag/internal/config"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/httpdl/httpdltest"
	"github.com/Roham1299/snag/internal/nativehost"
	"github.com/Roham1299/snag/internal/platform"
	"github.com/Roham1299/snag/internal/service"
	"github.com/Roham1299/snag/internal/streamdl/streamtest"
	"github.com/Roham1299/snag/internal/tools"
)

// extensionName also names its scripts' execution context in a page.
const extensionName = "Snag Download Manager"

func TestExtensionWithTheApp(t *testing.T) {
	browser := findBrowser(t)
	extension := buildExtension(t)
	root := t.TempDir()
	host := buildHost(t, root)
	app, socket := startApp(t, root)

	profile := filepath.Join(root, "profile")
	if _, err := nativehost.Register(host, []nativehost.Browser{{Name: "test", Family: nativehost.Chromium,
		ManifestDir: filepath.Join(profile, "NativeMessagingHosts")}}); err != nil {
		t.Fatal(err)
	}
	port := launch(t, browser, profile, extension, socket)

	files := httpdltest.NewServer(t)
	clip := make([]byte, 2<<20)
	_, _ = rand.Read(clip)
	clipURL := files.Add("/media/clip.mp4", clip)
	pages := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<!doctype html><title>Holiday clip</title><video src=%q controls muted autoplay></video>`, clipURL)
	}))
	t.Cleanup(pages.Close)

	tab := attach(t, port, "about:blank")
	tab.call(t, "Runtime.enable", nil)
	tab.call(t, "Page.navigate", map[string]any{"url": pages.URL + "/watch"})
	ext := tab.waitForExtension(t)

	t.Run("the extension reaches the app", func(t *testing.T) {
		var status struct{ OK bool }
		ext.ask(t, `{cmd: "status"}`, &status)
		if !status.OK {
			t.Fatal("the extension can't reach the app through snag-host")
		}
	})

	t.Run("the Download button on the video opens the card", func(t *testing.T) {
		checkDownloadButton(t, tab, "clip.mp4")
	})

	t.Run("a video on a page downloads", func(t *testing.T) {
		var d struct{ ID, Kind, Name string }
		ext.ask(t, fmt.Sprintf(`{cmd: "download", item: {url: %q, kind: "http", media: "video"}}`, clipURL), &d)
		if d.Kind != "http" || d.Name != "Holiday clip.mp4" {
			t.Errorf("the extension got %+v back", d)
		}
		done := waitStatus(t, app, d.ID, engine.StatusDone, engine.StatusError)
		if done.Status != engine.StatusDone || filepath.Base(done.Path) != "Holiday clip.mp4" {
			t.Fatalf("download ended %q at %q: %s", done.Status, done.Path, done.Error)
		}
		if got, _ := os.ReadFile(done.Path); len(got) != len(clip) {
			t.Errorf("file has %d bytes, want %d", len(got), len(clip))
		}
	})

	t.Run("a stream's qualities, and downloading one", func(t *testing.T) {
		if tools.Find("").YtDLP == "" {
			t.Skip("needs yt-dlp: run `task tools`, or set SNAG_TOOLS")
		}
		stream := streamtest.NewServer(t)
		var found struct {
			Supported bool
			Choices   []struct{ Label, Format string }
		}
		ext.ask(t, fmt.Sprintf(`{cmd: "probeStream", url: %q}`, stream.URL+"/e2e/master.m3u8"), &found)
		if !found.Supported || len(found.Choices) != 3 || found.Choices[0].Label != "1080p" {
			t.Fatalf("stream offers %+v", found)
		}
		var d struct{ ID, Kind string }
		ext.ask(t, fmt.Sprintf(`{cmd: "download", item: {url: %q, kind: "ytdlp", media: "hls", format: %q, label: "1080p"}}`,
			stream.URL+"/e2e/master.m3u8", found.Choices[0].Format), &d)
		if d.Kind != "ytdlp" {
			t.Errorf("kind %q, want ytdlp", d.Kind)
		}
		deadline := time.Now().Add(60 * time.Second)
		for len(stream.Completed("/e2e/")) < 3 {
			if time.Now().After(deadline) {
				got, _ := app.Engine.Get(d.ID)
				t.Fatalf("the stream isn't downloading: %+v", got)
			}
			time.Sleep(200 * time.Millisecond)
		}
		_ = app.Engine.Remove(d.ID, false)
	})
}

// inPage is the extension's in-page script in one tab.
type inPage struct {
	tab     *cdp
	context int
}

func (c *cdp) waitForExtension(t *testing.T) *inPage {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		for _, params := range c.eventsNamed("Runtime.executionContextCreated") {
			var e struct {
				Context struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"context"`
			}
			if json.Unmarshal(params, &e) == nil && e.Context.Name == extensionName {
				return &inPage{tab: c, context: e.Context.ID}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("the extension's script never ran in the page")
	return nil
}

// ask messages the extension's background as its in-page script does, and decodes the answer.
func (e *inPage) ask(t *testing.T, message string, into any) {
	t.Helper()
	var reply struct {
		OK    bool            `json:"ok"`
		Value json.RawMessage `json:"value"`
		Error string          `json:"error"`
	}
	e.tab.evaluate(t, e.context, "chrome.runtime.sendMessage("+message+")", &reply)
	if !reply.OK {
		t.Fatalf("%s: %s", message, reply.Error)
	}
	if into != nil {
		if err := json.Unmarshal(reply.Value, into); err != nil {
			t.Fatalf("%s answered %s: %v", message, reply.Value, err)
		}
	}
}

func waitStatus(t *testing.T, app *service.Service, id string, statuses ...engine.Status) engine.Download {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		d, err := app.Engine.Get(id)
		if err == nil {
			for _, s := range statuses {
				if d.Status == s {
					return d
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("download %s never reached %v", id, statuses)
	return engine.Download{}
}

// findBrowser returns a Chromium-family browser: $SNAG_BROWSER, or one on the PATH.
func findBrowser(t *testing.T) string {
	if path := os.Getenv("SNAG_BROWSER"); path != "" {
		return path
	}
	for _, name := range []string{"chromium", "chromium-browser", "vivaldi", "chrome"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	cantRun(t, "needs a Chromium-family browser on the PATH, or $SNAG_BROWSER")
	return ""
}

func buildExtension(t *testing.T) string {
	dir := filepath.Join(t.TempDir(), "extension")
	if _, err := extension.Install(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func buildHost(t *testing.T, dir string) string {
	host := filepath.Join(dir, "snag-host")
	if out, err := exec.Command("go", "build", "-o", host, "github.com/Roham1299/snag/cmd/snag-host").CombinedOutput(); err != nil {
		t.Fatalf("building snag-host: %v\n%s", err, out)
	}
	return host
}

func startApp(t *testing.T, root string) (*service.Service, string) {
	socketDir, err := os.MkdirTemp("", "snag") // socket paths are limited to ~100 bytes
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "app.sock")
	app, err := service.Start(service.Options{
		Paths: platform.Paths{
			Config: filepath.Join(root, "settings.json"), State: filepath.Join(root, "list.json"),
			Work: filepath.Join(root, "work"), Downloads: filepath.Join(root, "downloads"), Socket: socket,
		},
		Tools:  tools.Find(""),
		Logger: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Update(func(c *config.Config) { c.Ask = false }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.Close(context.Background()) })
	return app, socket
}

// launch starts the browser headless with a fresh profile and returns its DevTools port.
func launch(t *testing.T, browser, profile, extension, socket string) int {
	cmd := exec.Command(browser, "--headless=new", "--user-data-dir="+profile, "--no-first-run",
		"--no-default-browser-check", "--no-sandbox", "--remote-debugging-port=0",
		"--disable-extensions-except="+extension, "--load-extension="+extension,
		"--disable-features=DisableLoadExtensionCommandLineSwitch",
		"--autoplay-policy=no-user-gesture-required", "--mute-audio", "about:blank")
	cmd.Env = append(os.Environ(), "SNAG_SOCKET="+socket)
	platform.StartGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = platform.StopGroup(cmd, true)
		_ = cmd.Wait()
	})

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(filepath.Join(profile, "DevToolsActivePort")); err == nil {
			line, _, _ := strings.Cut(string(data), "\n")
			if port, err := strconv.Atoi(strings.TrimSpace(line)); err == nil && port > 0 {
				return port
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("the browser didn't start")
	return 0
}

// cantRun skips the test, or fails it where it must run ($SNAG_REQUIRE_E2E, set in CI).
func cantRun(t *testing.T, why string) {
	t.Helper()
	if os.Getenv("SNAG_REQUIRE_E2E") != "" {
		t.Fatal(why)
	}
	t.Skip(why)
}
