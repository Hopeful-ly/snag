// Package nativehost registers the snag-host native messaging helper with installed browsers,
// via a manifest per browser family (referenced from the registry on Windows).
package nativehost

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Name is the helper's name, which the extension asks the browser for.
const Name = "snag.host"

// The extension's IDs: Chromium's derives from the packing key, Firefox's is set in its manifest.
const (
	chromiumOrigin = "chrome-extension://dbkengnfcibdmedgidblleimphdigbli/"
	firefoxID      = "snag@localhost"
)

// Family is a browser family; each reads its own manifest format.
type Family int

// The browser families.
const (
	Chromium Family = iota // Chrome, Chromium, Edge, Brave, Vivaldi, Opera
	Firefox
)

// Browser is one browser and where it looks for native messaging manifests.
type Browser struct {
	Name   string
	Family Family
	// Profile is the browser's folder; if missing, the browser is skipped.
	// "" means always register (Windows, where the registry key does no harm).
	Profile     string
	ManifestDir string
	// RegistryKey is a Windows key under HKEY_CURRENT_USER whose default value points to the manifest.
	RegistryKey string
}

func (b Browser) installed() bool {
	if b.Profile == "" {
		return true
	}
	_, err := os.Stat(b.Profile)
	return err == nil
}

func (b Browser) manifestPath() string {
	file := Name + ".json"
	if b.RegistryKey != "" {
		// One folder holds both families' manifests on Windows.
		file = Name + map[Family]string{Chromium: ".chromium", Firefox: ".firefox"}[b.Family] + ".json"
	}
	return filepath.Join(b.ManifestDir, file)
}

func manifest(family Family, host string) ([]byte, error) {
	m := map[string]any{
		"name":        Name,
		"description": "Snag download manager",
		"path":        host,
		"type":        "stdio",
	}
	if family == Firefox {
		m["allowed_extensions"] = []string{firefoxID}
	} else {
		m["allowed_origins"] = []string{chromiumOrigin}
	}
	return json.MarshalIndent(m, "", "  ")
}

// Register registers the helper at host with the installed browsers and returns their names.
func Register(host string, browsers []Browser) ([]string, error) {
	var done []string
	var errs []error
	for _, b := range browsers {
		if !b.installed() {
			continue
		}
		if err := register(b, host); err != nil {
			errs = append(errs, err)
			continue
		}
		done = append(done, b.Name)
	}
	return done, errors.Join(errs...)
}

func register(b Browser, host string) error {
	data, err := manifest(b.Family, host)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(b.ManifestDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(b.manifestPath(), data, 0o644); err != nil {
		return err
	}
	if b.RegistryKey != "" {
		return setRegistry(b.RegistryKey, b.manifestPath())
	}
	return nil
}

// Unregister removes the helper from browsers.
func Unregister(browsers []Browser) error {
	var errs []error
	for _, b := range browsers {
		if err := os.Remove(b.manifestPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
		if b.RegistryKey != "" {
			errs = append(errs, deleteRegistry(b.RegistryKey))
		}
	}
	return errors.Join(errs...)
}

// RegisterNextTo registers the snag-host next to exe with every installed browser.
func RegisterNextTo(exe string) ([]string, error) {
	host, err := exec.LookPath(filepath.Join(filepath.Dir(exe), "snag-host"))
	if err != nil {
		return nil, fmt.Errorf("the browser helper isn't next to Snag: %w", err)
	}
	browsers, err := Browsers()
	if err != nil {
		return nil, err
	}
	return Register(host, browsers)
}
