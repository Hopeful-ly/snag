// Package config holds Snag's settings: their defaults, their limits, and the file they live in.
package config

import (
	"slices"
	"strings"

	"github.com/Roham1299/snag/internal/naming"
	"github.com/Roham1299/snag/internal/store"
)

const (
	maxConnections = 32
	maxActive      = 10
	maxRecentDirs  = 6
)

// Config is everything the user can set.
type Config struct {
	DownloadDir string `json:"downloadDir"`
	// Connections is per download.
	Connections int `json:"connections"`
	// MaxActive is the Main queue's concurrency.
	MaxActive int `json:"maxActive"`
	// Queues has Main first.
	Queues []Queue `json:"queues"`
	Notify bool    `json:"notify"`
	Ask    bool    `json:"ask"`
	Naming Naming  `json:"naming"`
	// RecentDirs is newest first.
	RecentDirs []string `json:"recentDirs,omitempty"`
	// TMDBKey is a TMDB v3 key or v4 token, used to list episodes on sites whose pages don't.
	// Never sent to the browser extension.
	TMDBKey string `json:"tmdbKey,omitempty"`
	// BrowserConnected stops the setup screen opening by itself once the extension has connected.
	BrowserConnected bool `json:"browserConnected,omitempty"`
	// StartAtLoginSet records that autostart was enabled on first run, so turning it off sticks.
	// Whether it is on is read from the system (platform.Autostart).
	StartAtLoginSet bool `json:"startAtLoginSet,omitempty"`
}

// Naming holds a naming.Template per kind of list.
type Naming struct {
	Songs    naming.Template `json:"songs"`
	Videos   naming.Template `json:"videos"`
	Episodes naming.Template `json:"episodes"`
}

// DefaultNaming is the naming used until the user changes it.
var DefaultNaming = Naming{
	Songs:    "{artist} - {title}",
	Videos:   "{n} - {title}",
	Episodes: "{show} S{season}E{episode} - {title}",
}

// normal resets templates that can't name files to their defaults.
func (n Naming) normal() Naming {
	fix := func(t *naming.Template, def naming.Template) {
		*t = naming.Template(strings.TrimSpace(string(*t)))
		if t.Check() != nil {
			*t = def
		}
	}
	fix(&n.Songs, DefaultNaming.Songs)
	fix(&n.Videos, DefaultNaming.Videos)
	fix(&n.Episodes, DefaultNaming.Episodes)
	return n
}

// Default returns the settings for a new install.
func Default(downloadDir string) Config {
	return Config{
		DownloadDir: downloadDir,
		Connections: 8,
		MaxActive:   3,
		Notify:      true,
		Ask:         true,
		Naming:      DefaultNaming,
	}
}

// Load reads the settings at path over defaults. On error it still returns usable settings.
func Load(path string, defaults Config) (Config, error) {
	c := defaults
	err := store.NewFile[Config](path).Load(&c)
	if c.DownloadDir == "" {
		c.DownloadDir = defaults.DownloadDir
	}
	return c.Normalized(), err
}

// Save writes c to path.
func Save(path string, c Config) error {
	return store.NewFile[Config](path).Save(c.Normalized())
}

// AddRecentDir puts dir first among the recent folders.
func (c *Config) AddRecentDir(dir string) {
	recent := []string{dir}
	for _, d := range c.RecentDirs {
		if d != dir {
			recent = append(recent, d)
		}
	}
	c.RecentDirs = recent[:min(len(recent), maxRecentDirs)]
}

// Normalized returns c with every value clamped to its limits.
func (c Config) Normalized() Config {
	c.Connections = clamp(c.Connections, 1, maxConnections)
	c.MaxActive = clamp(c.MaxActive, 1, maxActive)
	c.Queues = c.normalQueues()
	c.RecentDirs = slices.Clone(c.RecentDirs)
	c.TMDBKey = strings.TrimSpace(c.TMDBKey)
	c.Naming = c.Naming.normal()
	return c
}

func clamp(v, lo, hi int) int {
	return max(lo, min(v, hi))
}
