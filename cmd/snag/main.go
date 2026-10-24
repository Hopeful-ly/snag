// Command snag runs the Snag desktop app.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Roham1299/snag/frontend"
	"github.com/Roham1299/snag/internal/app"
	"github.com/Roham1299/snag/internal/feed"
	"github.com/Roham1299/snag/internal/nativehost"
	"github.com/Roham1299/snag/internal/platform"
	"github.com/Roham1299/snag/internal/service"
	"github.com/Roham1299/snag/internal/tools"
	"github.com/Roham1299/snag/internal/version"
)

const shutdownTimeout = 10 * time.Second

// The first feed and yt-dlp check is delayed to stay out of the way at login.
const (
	firstCheckAfter = 30 * time.Second
	checkEvery      = 24 * time.Hour
)

func main() {
	background := flag.Bool("background", false, "start in the tray, without the window")
	atLogin := flag.Bool("at-login", false, "start in the tray, as the user logs in")
	registerBrowsers := flag.Bool("register-browsers", false, "make the browser extension use this copy of Snag, then exit")
	unregisterBrowsers := flag.Bool("unregister-browsers", false, "undo --register-browsers and starting at login (uninstallers), then exit")
	flag.Parse()
	var err error
	switch {
	case *registerBrowsers:
		err = register()
	case *unregisterBrowsers:
		err = unregister()
	default:
		err = run(*background || *atLogin, *atLogin)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func register() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	names, err := nativehost.RegisterNextTo(exe)
	if len(names) > 0 {
		fmt.Println("Snag's browser helper is set up for:", strings.Join(names, ", "))
	} else if err == nil {
		fmt.Println("No supported browser found.")
	}
	return err
}

// unregister also turns off starting at login, for uninstallers.
func unregister() error {
	if a, ok := platform.AutostartFor(); ok {
		if err := a.Set(false); err != nil {
			log.Printf("couldn't stop Snag starting at login: %v", err)
		}
	}
	browsers, err := nativehost.Browsers()
	if err != nil {
		return err
	}
	return nativehost.Unregister(browsers)
}

func run(background, atLogin bool) error {
	paths, err := platform.DefaultPaths()
	if err != nil {
		return err
	}
	// First, so a second instance hands over to the running one before touching anything.
	ui := app.New(frontend.Assets(), app.Options{
		Background: background,
	})

	// Re-registering on every start follows the install through updates and moves.
	if version.IsPackaged() {
		if err := register(); err != nil {
			log.Printf("couldn't set the browsers up: %v", err)
		}
	}

	updatable := tools.PrepareYtDLP(paths.Tools)
	checker, err := feed.NewChecker(feed.URL, feed.PublicKey, filepath.Join(filepath.Dir(paths.State), "feed.json"))
	if err != nil {
		return err
	}
	svc, err := service.Start(service.Options{Paths: paths, Tools: tools.Find(paths.Tools), AtLogin: atLogin})
	if err != nil {
		return err
	}
	if a, ok := platform.AutostartFor(); ok {
		svc.UseAutostart(a)
	}
	updates, stopUpdates := context.WithCancel(context.Background())
	defer stopUpdates()
	go keepUpToDate(updates, checker, updatable, ui)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		svc.Close(ctx)
	}()
	ui.Attach(svc)

	// Quit cleanly on logout or shutdown so downloads save their progress.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		ui.Quit()
	}()

	return ui.Run()
}

// keepUpToDate updates yt-dlp to the version the feed names and reports newer Snag releases.
func keepUpToDate(ctx context.Context, checker *feed.Checker, ytdlp string, ui *app.App) {
	wait := firstCheckAfter
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = checkEvery
		latest := checker.Check(ctx)
		tools.UpdateYtDLP(ctx, ytdlp, latest.YtDLP)
		if version.IsPackaged() {
			newer := latest.Newer(version.String())
			ui.SetUpdate(newer, checker.Tell(newer))
		}
	}
}
