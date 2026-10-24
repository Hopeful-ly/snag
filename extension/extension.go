// Package extension holds the Snag browser extension, built into the app, so the app can keep a
// ready-to-load copy of it in its own folder: browsers outside their stores only run extensions
// loaded from a folder ("Load unpacked"), and an app update brings the extension's update along.
package extension

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// build/ is the extension's scripts and icons, made from src/ by `npm run build` (task
// extension:build). Without a build only its placeholder is there, and the tests say so.
//
//go:embed all:build manifests
var files embed.FS

// Chromium returns the extension for Chromium browsers (Chrome, Edge, Brave, Vivaldi, Opera):
// file names and contents. Its manifest carries the key that fixes the extension's id, which the
// browser helper's registration names.
func Chromium() (map[string][]byte, error) {
	out, err := scripts()
	if err != nil {
		return nil, err
	}
	manifest, err := files.ReadFile("manifests/chromium.json")
	if err != nil {
		return nil, err
	}
	key, err := files.ReadFile("manifests/chromium.key.txt")
	if err != nil {
		return nil, err
	}
	out["manifest.json"] = bytes.ReplaceAll(manifest, []byte("__KEY__"), bytes.TrimSpace(key))
	return out, nil
}

// Firefox returns the extension for Firefox. Mozilla signs each version once, so a release
// stamps its own version on the manifest; updateURL, if set, is where Firefox looks for newer
// signed versions (Snag's website), as add-ons from outside Mozilla's store update no other way.
func Firefox(version, updateURL string) (map[string][]byte, error) {
	out, err := scripts()
	if err != nil {
		return nil, err
	}
	data, err := files.ReadFile("manifests/firefox.json")
	if err != nil {
		return nil, err
	}
	var manifest firefoxManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	if version != "" {
		manifest["version"] = version
	}
	if updateURL != "" {
		gecko, err := manifest.gecko()
		if err != nil {
			return nil, err
		}
		gecko["update_url"] = updateURL
	}
	if out["manifest.json"], err = json.MarshalIndent(manifest, "", "  "); err != nil {
		return nil, err
	}
	return out, nil
}

// FirefoxID is the id Firefox knows the extension by, set in its manifest.
func FirefoxID() (string, error) {
	data, err := files.ReadFile("manifests/firefox.json")
	if err != nil {
		return "", err
	}
	var manifest firefoxManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", err
	}
	gecko, err := manifest.gecko()
	if err != nil {
		return "", err
	}
	id, _ := gecko["id"].(string)
	return id, nil
}

type firefoxManifest map[string]any

// gecko returns the manifest's Firefox settings (browser_specific_settings.gecko).
func (m firefoxManifest) gecko() (map[string]any, error) {
	settings, _ := m["browser_specific_settings"].(map[string]any)
	gecko, _ := settings["gecko"].(map[string]any)
	if gecko == nil {
		return nil, errors.New("the Firefox manifest has no browser_specific_settings.gecko")
	}
	return gecko, nil
}

// scripts returns the files both browsers share: the scripts, pages and icons.
func scripts() (map[string][]byte, error) {
	out := map[string][]byte{}
	for _, dir := range []string{"build"} { // scripts and icons; packaging/icons is the original
		entries, err := fs.ReadDir(files, dir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			data, err := files.ReadFile(path.Join(dir, e.Name()))
			if err != nil {
				return nil, err
			}
			out[e.Name()] = data
		}
	}
	return out, nil
}

// Install writes the Chromium extension into dir, leaving files that are already right alone and
// removing ones it no longer has. It reports whether anything changed, which means a browser that
// loaded the folder needs a restart or the extension's reload button.
func Install(dir string) (changed bool, err error) {
	want, err := Chromium()
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	var errs []error
	for _, e := range entries {
		if _, keep := want[e.Name()]; !keep && !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			changed = true
			errs = append(errs, os.Remove(filepath.Join(dir, e.Name())))
		}
	}
	for name, data := range want {
		target := filepath.Join(dir, name)
		if have, err := os.ReadFile(target); err == nil && bytes.Equal(have, data) {
			continue
		}
		changed = true
		errs = append(errs, os.WriteFile(target, data, 0o644))
	}
	return changed, errors.Join(errs...)
}
