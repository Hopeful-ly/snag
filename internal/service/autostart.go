package service

import (
	"errors"

	"github.com/Roham1299/snag/internal/config"
	"github.com/Roham1299/snag/internal/platform"
)

var errNoAutostart = errors.New("this copy of Snag can't start at login (a development build, or SNAG_HOME is set)")

// UseAutostart sets how Snag starts at login and turns it on the first time only, so a user
// who turns it off keeps it off.
func (s *Service) UseAutostart(a platform.Autostart) {
	s.mu.Lock()
	s.autostart = &a
	s.mu.Unlock()
	if s.Config().StartAtLoginSet {
		return
	}
	on, err := a.Enabled()
	if err == nil && !on {
		err = a.Set(true)
	}
	if err != nil {
		s.log.Warn("couldn't make Snag start at login", "err", err)
		return
	}
	s.log.Info("Snag starts at login from now on", "program", a.Program)
	if _, err := s.Update(func(c *config.Config) { c.StartAtLoginSet = true }); err != nil {
		s.log.Warn("couldn't save the settings", "err", err)
	}
}

// StartAtLogin reports whether Snag starts at login, and whether this Snag can at all.
func (s *Service) StartAtLogin() (on, available bool, err error) {
	s.mu.Lock()
	a := s.autostart
	s.mu.Unlock()
	if a == nil {
		return false, false, nil
	}
	on, err = a.Enabled()
	return on, true, err
}

// SetStartAtLogin makes Snag start at login, or not.
func (s *Service) SetStartAtLogin(on bool) error {
	s.mu.Lock()
	a := s.autostart
	s.mu.Unlock()
	if a == nil {
		return errNoAutostart
	}
	if err := a.Set(on); err != nil {
		return err
	}
	_, err := s.Update(func(c *config.Config) { c.StartAtLoginSet = true })
	return err
}

// StartAtLoginManual names a desktop that ignores Snag's autostart entry and the command to add
// to its own startup; "" when the entry is enough.
func (s *Service) StartAtLoginManual() (desktop, command string) {
	s.mu.Lock()
	a := s.autostart
	s.mu.Unlock()
	if a == nil {
		return "", ""
	}
	return a.Manual()
}
