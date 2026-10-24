package app

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/Roham1299/snag/internal/config"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/platform"
	"github.com/Roham1299/snag/packaging/icons"
)

var (
	trayBusyIcon = withDot(icons.Tray)
)

const (
	trayRefreshDelay = 500 * time.Millisecond
	trayMaxRunning   = 5
)

type tray struct {
	app  *App
	icon *application.SystemTray
	menu *application.Menu

	mu      sync.Mutex
	pending bool
	busy    bool
}

func newTray(a *App) *tray {
	t := &tray{app: a, icon: a.wails.SystemTray.New(), menu: a.wails.NewMenu()}
	if runtime.GOOS == "darwin" {
		t.icon.SetTemplateIcon(icons.TrayTemplate)
	} else {
		t.icon.SetIcon(icons.Tray)
	}
	t.setTooltip(name)
	t.icon.OnClick(a.show)
	t.build(nil, config.Config{})
	t.icon.SetMenu(t.menu)
	return t
}

// changed schedules a refresh, coalescing bursts of changes.
func (t *tray) changed() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pending {
		return
	}
	t.pending = true
	time.AfterFunc(trayRefreshDelay, t.refresh)
}

func (t *tray) refresh() {
	t.mu.Lock()
	t.pending = false
	t.mu.Unlock()
	svc, err := t.app.service()
	if err != nil {
		return
	}
	list, settings := svc.Engine.List(), svc.Config()
	application.InvokeSync(func() {
		t.build(list, settings)
		t.menu.Update()
		t.show(list)
	})
}

func (t *tray) build(list []engine.Download, settings config.Config) {
	a := t.app
	running, paused := 0, 0
	t.menu.Clear()
	t.menu.Add("Open Snag").OnClick(func(*application.Context) { a.show() })
	for _, d := range list {
		switch d.Status {
		case engine.StatusDownloading, engine.StatusQueued:
			running++
			if d.Status == engine.StatusDownloading && running <= trayMaxRunning {
				t.menu.Add(trayLine(d)).OnClick(func(*application.Context) { a.show() })
			}
		case engine.StatusPaused, engine.StatusError:
			paused++
		}
	}
	t.menu.AddSeparator()
	t.menu.Add("Pause all").SetEnabled(running > 0).OnClick(func(*application.Context) { a.withEngine((*engine.Engine).PauseAll) })
	t.menu.Add("Resume all").SetEnabled(paused > 0).OnClick(func(*application.Context) { a.withEngine((*engine.Engine).ResumeAll) })
	t.menu.AddSeparator()
	t.menu.AddCheckbox("Ask where to save", settings.Ask).OnClick(func(ctx *application.Context) {
		if svc, err := a.service(); err == nil {
			_, _ = svc.Update(func(c *config.Config) { c.Ask = !c.Ask })
		}
	})
	t.menu.Add("Open downloads folder").OnClick(func(*application.Context) {
		if err := os.MkdirAll(settings.DownloadDir, 0o755); err == nil {
			_ = platform.Open(settings.DownloadDir)
		}
	})
	t.menu.AddSeparator()
	t.menu.Add("Quit Snag").OnClick(func(*application.Context) { a.Quit() })
}

func (t *tray) show(list []engine.Download) {
	var speed int64
	running := 0
	for _, d := range list {
		if d.Status == engine.StatusDownloading {
			running++
			speed += d.Speed
		}
	}
	if running == 0 {
		t.setTooltip(name)
	} else {
		t.setTooltip(fmt.Sprintf("%s: %d downloading, %s/s", name, running, sizeText(speed)))
	}
	busy := running > 0
	if busy != t.busy && runtime.GOOS != "darwin" {
		t.busy = busy
		if busy {
			t.icon.SetIcon(trayBusyIcon)
		} else {
			t.icon.SetIcon(icons.Tray)
		}
	}
}

// Linux tray hosts show the label as the item's title and tooltip; others have a tooltip.
func (t *tray) setTooltip(text string) {
	if runtime.GOOS == "linux" {
		t.icon.SetLabel(text)
	} else {
		t.icon.SetTooltip(text)
	}
}

func (a *App) withEngine(do func(*engine.Engine)) {
	if svc, err := a.service(); err == nil {
		do(svc.Engine)
	}
}

// trayLine formats "Name — 45% · 2.1 MB/s".
func trayLine(d engine.Download) string {
	label := []rune(d.Name())
	if len(label) > 40 {
		label = append(label[:39], '…')
	}
	line := string(label)
	if d.Size > 0 {
		line += fmt.Sprintf(" — %d%%", min(100, d.Done*100/d.Size))
	}
	if d.Speed > 0 {
		line += " · " + sizeText(d.Speed) + "/s"
	}
	return line
}

// sizeText formats bytes as the window does: "850 KB", "2.1 MB", "14 GB".
func sizeText(bytes int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value, i := float64(bytes), 0
	for value >= 1024 && i < len(units)-1 {
		value /= 1024
		i++
	}
	switch {
	case i >= 2 && value < 10:
		return fmt.Sprintf("%.1f %s", value, units[i])
	default:
		return fmt.Sprintf("%.0f %s", value, units[i])
	}
}

func withDot(icon []byte) []byte {
	src, err := png.Decode(bytes.NewReader(icon))
	if err != nil {
		slog.Warn("couldn't read the tray icon", "err", err)
		return icon
	}
	b := src.Bounds()
	img := image.NewRGBA(b)
	draw.Draw(img, b, src, b.Min, draw.Src)
	r := b.Dx() / 5
	cx, cy := b.Max.X-r-1, b.Max.Y-r-1
	green := color.RGBA{0x4a, 0xde, 0x80, 0xff}
	for y := cy - r - 2; y <= cy+r+2; y++ {
		for x := cx - r - 2; x <= cx+r+2; x++ {
			d := (x-cx)*(x-cx) + (y-cy)*(y-cy)
			switch {
			case d <= r*r:
				img.Set(x, y, green)
			case d <= (r+2)*(r+2):
				img.Set(x, y, color.RGBA{0x12, 0x14, 0x18, 0xff}) // dark ring around the dot
			}
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return icon
	}
	return out.Bytes()
}
