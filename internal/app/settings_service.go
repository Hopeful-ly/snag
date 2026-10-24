package app

import (
	"os"
	"path/filepath"

	"github.com/Roham1299/snag/internal/config"
)

// maxPlaces caps the folders the Save dialog offers.
const maxPlaces = 6

// SettingsService backs the settings UI.
type SettingsService struct {
	app *App
}

// Get returns the settings.
func (s *SettingsService) Get() (config.Config, error) {
	svc, err := s.app.service()
	if err != nil {
		return config.Config{}, err
	}
	return svc.Config(), nil
}

// Set replaces the settings and returns them as saved, with numbers clamped to their limits.
func (s *SettingsService) Set(settings config.Config) (config.Config, error) {
	svc, err := s.app.service()
	if err != nil {
		return config.Config{}, err
	}
	return svc.Update(func(c *config.Config) { *c = settings })
}

// ChooseFolder asks for a folder, starting at start; "" when the user cancels.
func (s *SettingsService) ChooseFolder(start string) (string, error) {
	return s.app.wails.Dialog.OpenFile().
		SetTitle("Choose a folder").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		SetDirectory(start).
		PromptForSingleSelection()
}

// Places are the folders the Save dialog offers: downloads, videos, music and recent folders.
func (s *SettingsService) Places() ([]string, error) {
	settings, err := s.Get()
	if err != nil {
		return nil, err
	}
	home, _ := os.UserHomeDir()
	candidates := []string{settings.DownloadDir}
	for _, dir := range []string{"Videos", "Movies", "Music"} {
		if info, err := os.Stat(filepath.Join(home, dir)); err == nil && info.IsDir() {
			candidates = append(candidates, filepath.Join(home, dir))
		}
	}
	candidates = append(candidates, settings.RecentDirs...)

	var places []string
	seen := map[string]bool{}
	for _, p := range candidates {
		if p != "" && !seen[p] && len(places) < maxPlaces {
			seen[p] = true
			places = append(places, p)
		}
	}
	return places, nil
}

// Home returns the user's home folder, which the window shortens to "~".
func (s *SettingsService) Home() string {
	home, _ := os.UserHomeDir()
	return home
}

// StartAtLogin is whether Snag starts at login, and whether this copy can (dev builds can't).
type StartAtLogin struct {
	On        bool `json:"on"`
	Available bool `json:"available"`
	// Desktop names a desktop that ignores the autostart entry (Hyprland, Sway…), and Command
	// what to add to its own startup; "" when the entry is enough.
	Desktop string `json:"desktop"`
	Command string `json:"command"`
}

// StartAtLogin reports whether Snag starts at login, read from the system since the user may
// have changed it there.
func (s *SettingsService) StartAtLogin() (StartAtLogin, error) {
	svc, err := s.app.service()
	if err != nil {
		return StartAtLogin{}, err
	}
	on, available, err := svc.StartAtLogin()
	desktop, command := svc.StartAtLoginManual()
	return StartAtLogin{On: on, Available: available, Desktop: desktop, Command: command}, err
}

// SetStartAtLogin turns starting at login on or off and returns the new state.
func (s *SettingsService) SetStartAtLogin(on bool) (StartAtLogin, error) {
	svc, err := s.app.service()
	if err != nil {
		return StartAtLogin{}, err
	}
	if err := svc.SetStartAtLogin(on); err != nil {
		return StartAtLogin{}, err
	}
	return s.StartAtLogin()
}
