package extension

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if _, err := files.ReadFile("build/background.js"); err != nil {
		fmt.Fprintln(os.Stderr, "extension/build is empty: build the extension first (task extension:build)")
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestChromiumHasEverythingItsManifestNames(t *testing.T) {
	files, err := Chromium()
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Key        string `json:"key"`
		Background struct {
			ServiceWorker string `json:"service_worker"`
		} `json:"background"`
		ContentScripts []struct {
			JS []string `json:"js"`
		} `json:"content_scripts"`
		Action struct {
			DefaultPopup string            `json:"default_popup"`
			DefaultIcon  map[string]string `json:"default_icon"`
		} `json:"action"`
		Icons map[string]string `json:"icons"`
	}
	if err := json.Unmarshal(files["manifest.json"], &manifest); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if manifest.Key == "" || strings.Contains(manifest.Key, "__KEY__") {
		t.Errorf("manifest key = %q", manifest.Key)
	}
	if manifest.Action.DefaultPopup != "" {
		t.Error("action must not open a popup")
	}
	named := []string{manifest.Background.ServiceWorker}
	for _, cs := range manifest.ContentScripts {
		named = append(named, cs.JS...)
	}
	for _, icon := range manifest.Icons {
		named = append(named, icon)
	}
	for _, icon := range manifest.Action.DefaultIcon {
		named = append(named, icon)
	}
	for _, name := range named {
		if _, ok := files[name]; !ok {
			t.Errorf("the manifest names %q, which isn't there", name)
		}
	}
}

func TestInstallKeepsTheFolderCurrent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "old.js"), []byte("gone"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := Install(dir)
	if err != nil || !changed {
		t.Fatalf("first install: changed %v, %v", changed, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "old.js")); !os.IsNotExist(err) {
		t.Error("a file the extension no longer has was kept")
	}
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		t.Error("no manifest")
	}
	if changed, err := Install(dir); err != nil || changed {
		t.Errorf("installing the same again: changed %v, %v", changed, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "background.js"), []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, _ := Install(dir); !changed {
		t.Error("an edited file wasn't restored")
	}
}

func TestFirefoxIsStampedAndComplete(t *testing.T) {
	files, err := Firefox("1.2.3", "https://example.com/firefox/updates.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version  string `json:"version"`
		Settings struct {
			Gecko struct {
				ID        string `json:"id"`
				UpdateURL string `json:"update_url"`
			} `json:"gecko"`
		} `json:"browser_specific_settings"`
		Background struct {
			Scripts []string `json:"scripts"`
		} `json:"background"`
		ContentScripts []struct {
			JS []string `json:"js"`
		} `json:"content_scripts"`
		BrowserAction struct {
			DefaultPopup string            `json:"default_popup"`
			DefaultIcon  map[string]string `json:"default_icon"`
		} `json:"browser_action"`
		Icons map[string]string `json:"icons"`
	}
	if err := json.Unmarshal(files["manifest.json"], &manifest); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	id, err := FirefoxID()
	if manifest.Version != "1.2.3" || manifest.Settings.Gecko.UpdateURL != "https://example.com/firefox/updates.json" ||
		err != nil || manifest.Settings.Gecko.ID != id {
		t.Errorf("version %q, update_url %q, id %q (FirefoxID %q, %v)", manifest.Version,
			manifest.Settings.Gecko.UpdateURL, manifest.Settings.Gecko.ID, id, err)
	}
	if manifest.BrowserAction.DefaultPopup != "" {
		t.Error("browser_action must not open a popup")
	}
	named := append([]string{}, manifest.Background.Scripts...)
	for _, cs := range manifest.ContentScripts {
		named = append(named, cs.JS...)
	}
	for _, icon := range manifest.Icons {
		named = append(named, icon)
	}
	for _, icon := range manifest.BrowserAction.DefaultIcon {
		named = append(named, icon)
	}
	for _, name := range named {
		if _, ok := files[name]; !ok {
			t.Errorf("the manifest names %q, which isn't there", name)
		}
	}

	plain, err := Firefox("", "")
	if err != nil || strings.Contains(string(plain["manifest.json"]), "update_url") {
		t.Errorf("without an update address: %v\n%s", err, plain["manifest.json"])
	}
}

// The extension has one version, major.minor.patch, in both manifests and package.json: each
// change to it bumps it there.
func TestOneVersionEverywhere(t *testing.T) {
	versionOf := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var v struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(data, &v); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		return v.Version
	}
	want := versionOf("manifests/chromium.json")
	if !semver.MatchString(want) {
		t.Errorf("chromium.json's version %q isn't major.minor.patch", want)
	}
	for _, path := range []string{"manifests/firefox.json", "package.json"} {
		if got := versionOf(path); got != want {
			t.Errorf("%s says version %q, chromium.json %q", path, got, want)
		}
	}
}

var semver = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)
