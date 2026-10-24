// Package service is Snag's headless core: settings, the download engine, the prober and the
// local server the browser helper talks to.
package service

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/Roham1299/snag/extension"
	"github.com/Roham1299/snag/internal/applemusic"
	"github.com/Roham1299/snag/internal/config"
	"github.com/Roham1299/snag/internal/engine"
	"github.com/Roham1299/snag/internal/httpdl"
	"github.com/Roham1299/snag/internal/ipc"
	"github.com/Roham1299/snag/internal/links"
	"github.com/Roham1299/snag/internal/pagedl"
	"github.com/Roham1299/snag/internal/platform"
	"github.com/Roham1299/snag/internal/playlist"
	"github.com/Roham1299/snag/internal/probe"
	"github.com/Roham1299/snag/internal/protocol"
	"github.com/Roham1299/snag/internal/queues"
	"github.com/Roham1299/snag/internal/spotify"
	"github.com/Roham1299/snag/internal/store"
	"github.com/Roham1299/snag/internal/streamdl"
	"github.com/Roham1299/snag/internal/subscriptions"
	"github.com/Roham1299/snag/internal/tools"
	"github.com/Roham1299/snag/internal/watch"
	"github.com/Roham1299/snag/internal/ytdlp"
	"github.com/Roham1299/snag/internal/ytmusic"
)

// Options configure Start.
type Options struct {
	Paths  platform.Paths
	Tools  tools.Paths
	Logger *slog.Logger // nil for slog.Default()
	// AtLogin delays scheduled work by loginDelay so it doesn't compete with logging in.
	AtLogin bool
}

const loginDelay = time.Minute

// Service is the running headless core.
type Service struct {
	Engine    *engine.Engine
	Prober    *probe.Prober
	Playlists playlist.Lister

	paths     platform.Paths
	log       *slog.Logger
	handler   *protocol.Handler
	listener  net.Listener
	watch     *watch.Server // nil if it couldn't start
	playlists playlist.Store
	links     links.Store
	spotify   spotify.Client
	apple     applemusic.Client
	songs     ytmusic.Finder

	mu        sync.Mutex
	config    config.Config
	window    protocol.Window
	listeners []func(config.Config)
	contact   time.Time
	onContact []func()

	autostart          *platform.Autostart // nil when this Snag can't start at login
	subs               *subscriptions.Store
	checker            *subscriptions.Checker
	onSubscriptionNews []func(subscriptions.Subscription, []string)
	scheduler          *queues.Scheduler
	stopScheduler      context.CancelFunc
	onQueues           []func()
	onCountdown        []func(*Countdown)
	countdown          *countdown
	quit               func()
}

// Start loads settings and downloads, resumes them, and listens for the browser helper.
func Start(opts Options) (*Service, error) {
	s := &Service{paths: opts.Paths, log: opts.Logger}
	if s.log == nil {
		s.log = slog.Default()
	}
	var err error
	s.config, err = config.Load(opts.Paths.Config, config.Default(opts.Paths.Downloads))
	if err != nil {
		s.log.Warn("couldn't read the settings; using defaults", "err", err)
	}

	if opts.Paths.Extension != "" {
		if _, err := extension.Install(opts.Paths.Extension); err != nil {
			s.log.Warn("couldn't update the browser extension's folder", "dir", opts.Paths.Extension, "err", err)
		}
	}

	s.listener, err = ipc.Listen(opts.Paths.Socket)
	if err != nil {
		return nil, err
	}
	s.Engine = engine.New(engine.Options{
		Store: store.NewFile[[]engine.Download](opts.Paths.State),
		Runners: map[engine.Kind]engine.Runner{
			engine.KindHTTP: httpdl.New(),
			// pagedl resolves streaming-site pages in the browser and Spotify/Apple Music
			// songs on YouTube Music first.
			engine.KindStream: pagedl.Runner{Resolver: s, Next: streamdl.New(opts.Tools)},
		},
		WorkRoot:  opts.Paths.Work,
		Settings:  engineSettings(s.config),
		Logger:    s.log,
		SameDrive: platform.SameDrive,
		Hide:      platform.Hide,
		OnQueue:   s.queueEvent,

		FindChecksum: newChecksumFinder().Find,
	})
	s.scheduler = queues.New(s.Engine, func() []config.Queue { return s.Config().Queues })
	var schedulerCtx context.Context
	schedulerCtx, s.stopScheduler = context.WithCancel(context.Background())
	if opts.AtLogin {
		go s.scheduler.RunAfter(schedulerCtx, loginDelay)
	} else {
		go s.scheduler.Run(schedulerCtx)
	}
	s.Prober = probe.New(opts.Tools)
	s.songs = ytmusic.New(opts.Tools.YtDLP, ytdlp.Programs{FFmpeg: opts.Tools.FFmpeg, Node: opts.Tools.Node})
	s.Playlists = playlist.Listers{
		playlist.Spotify{Client: s.spotify},
		playlist.AppleMusic{Client: s.apple},
		playlist.Episodes{Key: func() string { return s.Config().TMDBKey }},
		playlist.Feed{},
		playlist.YtDLP{Prober: s.Prober},
	}
	s.subs = subscriptions.Open(filepath.Join(filepath.Dir(opts.Paths.State), "subscriptions.json"), s.log)
	s.startSubscriptions(schedulerCtx, opts.AtLogin)
	if s.watch, err = watch.Start(s.Engine, opts.Tools.FFmpeg, s.log); err != nil {
		s.log.Warn("can't stream downloads to players; watching opens the file instead", "err", err)
	}
	handler := protocol.New(protocol.Options{
		Engine:    s.Engine,
		Prober:    s.Prober,
		Playlists: s,
		Links:     s,
		Settings:  s,
		Files:     s,
		Window:    s,
		HasYtDLP:  opts.Tools.YtDLP != "",
		Contact:   s.browserContact,
	})
	s.mu.Lock()
	s.handler = handler // the engine may already be asking Resolve for it
	s.mu.Unlock()
	go func() {
		if err := handler.ServeListener(s.listener); err != nil {
			s.log.Error("the browser helper's server stopped", "err", err)
		}
	}()
	return s, nil
}

// Resolve finds the stream a download's page plays; it implements pagedl.Resolver.
func (s *Service) Resolve(ctx context.Context, req pagedl.Request) (pagedl.Source, error) {
	if ref, ok := spotify.Parse(req.Page); ok && ref.Kind == spotify.Track {
		return s.findSong(ctx, ref, req.Fallback)
	}
	if ref, ok := applemusic.Parse(req.Page); ok && ref.One() {
		return s.findAppleSong(ctx, ref, req.Fallback)
	}
	page := req.Page
	s.mu.Lock()
	h := s.handler
	s.mu.Unlock()
	if h == nil {
		return pagedl.Source{}, pagedl.ErrNoBrowser
	}
	src, err := h.Resolve(ctx, page)
	return src, pagedl.BrowserError(page, err)
}

// Close stops listening and stops the downloads, saving progress so the next start resumes.
func (s *Service) Close(ctx context.Context) {
	s.stopScheduler()
	s.CancelCountdown()
	s.listener.Close()
	s.handler.Close()
	if s.watch != nil {
		_ = s.watch.Close()
	}
	s.Engine.Shutdown(ctx)
}

// Config returns the user's settings.
func (s *Service) Config() config.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

// Update changes, saves and applies the settings, and notifies listeners.
func (s *Service) Update(change func(*config.Config)) (config.Config, error) {
	s.mu.Lock()
	c := s.config
	change(&c)
	c = c.Normalized()
	if err := config.Save(s.paths.Config, c); err != nil {
		old := s.config
		s.mu.Unlock()
		return old, err
	}
	s.config = c
	listeners := slices.Clone(s.listeners)
	s.mu.Unlock()

	s.Engine.SetSettings(engineSettings(c))
	s.scheduler.Check() // a schedule may have changed
	s.handler.ConfigChanged(c)
	for _, listen := range listeners {
		listen(c)
	}
	return c, nil
}

// OnSettingsChanged calls f with the settings whenever they change.
func (s *Service) OnSettingsChanged(f func(config.Config)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, f)
}

// ExtensionDir is the unpacked browser extension's folder.
func (s *Service) ExtensionDir() string {
	return s.paths.Extension
}

// LastContact returns when the browser extension last made a request this run, or zero.
func (s *Service) LastContact() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.contact
}

// OnBrowserContact calls f on the browser extension's first request this run.
func (s *Service) OnBrowserContact(f func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onContact = append(s.onContact, f)
}

func (s *Service) browserContact() {
	s.mu.Lock()
	first := s.contact.IsZero()
	s.contact = time.Now()
	remembered := s.config.BrowserConnected
	listeners := slices.Clone(s.onContact)
	s.mu.Unlock()
	if !first {
		return
	}
	if !remembered {
		if _, err := s.Update(func(c *config.Config) { c.BrowserConnected = true }); err != nil {
			s.log.Warn("couldn't remember the browser connection", "err", err)
		}
	}
	for _, f := range listeners {
		f()
	}
}

// SetWindow sets the window the browser can bring up.
func (s *Service) SetWindow(w protocol.Window) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.window = w
}

// Show brings up the window, if there is one.
func (s *Service) Show(downloadID string) {
	s.mu.Lock()
	w := s.window
	s.mu.Unlock()
	if w != nil {
		w.Show(downloadID)
	}
}

// ShowPlaylist opens the playlist window, if there is a window.
func (s *Service) ShowPlaylist(id string) {
	s.mu.Lock()
	w := s.window
	s.mu.Unlock()
	if w != nil {
		w.ShowPlaylist(id)
	}
}

// Open opens a finished download's file with its usual program.
func (s *Service) Open(id string) error {
	d, err := s.Engine.Get(id)
	if err != nil {
		return err
	}
	if d.Status != engine.StatusDone {
		return errors.New("not finished yet")
	}
	return platform.Open(d.Path)
}

// Reveal shows a download's file in the file manager, or its folder while it has no file yet.
func (s *Service) Reveal(id string) error {
	d, err := s.Engine.Get(id)
	if err != nil {
		return err
	}
	if _, err := os.Stat(d.Path); d.Path != "" && err == nil {
		return platform.Reveal(d.Path)
	}
	if err := os.MkdirAll(d.Directory, 0o755); err != nil {
		return err
	}
	return platform.Open(d.Directory)
}

func engineSettings(c config.Config) engine.Settings {
	s := engine.Settings{
		DownloadDir: c.DownloadDir,
		Connections: c.Connections,
		MaxActive:   c.MaxActive,
		Ask:         c.Ask,
	}
	for _, q := range c.Queues {
		s.Queues = append(s.Queues, engine.QueueSettings{
			ID: q.ID, AtOnce: q.AtOnce, SpeedLimit: q.SpeedLimit,
			Retries: q.Retries, RetryWait: time.Duration(q.RetryWait) * time.Second,
			Fallback: q.Fallback,
		})
	}
	return s
}
