package platform

import (
	"errors"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// Task Manager's Startup apps page disables Run entries via StartupApproved without removing them.
const (
	runKey      = `Software\Microsoft\Windows\CurrentVersion\Run`
	approvedKey = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`
)

// autostartValue is a var so tests can use another name.
var autostartValue = "Snag"

func (a Autostart) enabled() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer k.Close()
	if _, _, err := k.GetStringValue(autostartValue); errors.Is(err, registry.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	// Its first byte is odd when Task Manager turned it off.
	if approved, err := registry.OpenKey(registry.CURRENT_USER, approvedKey, registry.QUERY_VALUE); err == nil {
		defer approved.Close()
		if state, _, err := approved.GetBinaryValue(autostartValue); err == nil && len(state) > 0 && state[0]%2 == 1 {
			return false, nil
		}
	}
	return true, nil
}

func (a Autostart) enable() error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	command := `"` + strings.ReplaceAll(a.Program, `"`, "") + `" ` + AtLoginFlag
	if err := k.SetStringValue(autostartValue, command); err != nil {
		return err
	}
	// Clear a Task Manager disable.
	if approved, err := registry.OpenKey(registry.CURRENT_USER, approvedKey, registry.SET_VALUE); err == nil {
		defer approved.Close()
		_ = approved.DeleteValue(autostartValue)
	}
	return nil
}

func (a Autostart) disable() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.DeleteValue(autostartValue); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}

// Manual is "" on Windows: the Run key is always read.
func (a Autostart) Manual() (desktop, command string) { return "", "" }
