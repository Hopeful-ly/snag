package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLoadMissingFileGivesDefaults(t *testing.T) {
	defaults := Default("/downloads")
	got, err := Load(filepath.Join(t.TempDir(), "config.json"), defaults)
	if err != nil {
		t.Fatal(err)
	}
	if got.DownloadDir != "/downloads" || got.Connections != 8 || got.MaxActive != 3 || !got.Ask ||
		got.Naming != DefaultNaming {
		t.Errorf("Load = %+v", got)
	}
}

func TestLoadMergesOverDefaultsAndClamps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"connections": 500, "maxActive": 0, "ask": false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path, Default("/downloads"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Connections != maxConnections || got.MaxActive != 1 || got.Ask || got.DownloadDir != "/downloads" {
		t.Errorf("Load = %+v", got)
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	want := Default("/elsewhere")
	want.Notify = false
	want.AddRecentDir("/a")
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path, Default("/downloads"))
	if err != nil {
		t.Fatal(err)
	}
	if got.DownloadDir != "/elsewhere" || got.Notify || !slices.Equal(got.RecentDirs, []string{"/a"}) {
		t.Errorf("Load = %+v", got)
	}
}

func TestAddRecentDir(t *testing.T) {
	var c Config
	for _, dir := range []string{"/1", "/2", "/3", "/4", "/5", "/6", "/7", "/3"} {
		c.AddRecentDir(dir)
	}
	want := []string{"/3", "/7", "/6", "/5", "/4", "/2"}
	if !slices.Equal(c.RecentDirs, want) {
		t.Errorf("RecentDirs = %v, want %v", c.RecentDirs, want)
	}
}
