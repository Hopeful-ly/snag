package store

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type settings struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestSaveAndLoad(t *testing.T) {
	f := NewFile[settings](filepath.Join(t.TempDir(), "sub", "settings.json"))
	if err := f.Save(settings{Name: "a", Count: 2}); err != nil {
		t.Fatal(err)
	}
	var got settings
	if err := f.Load(&got); err != nil {
		t.Fatal(err)
	}
	if got != (settings{Name: "a", Count: 2}) {
		t.Errorf("loaded %+v", got)
	}
}

func TestLoadKeepsDefaults(t *testing.T) {
	dir := t.TempDir()
	missing := NewFile[settings](filepath.Join(dir, "missing.json"))
	got := settings{Name: "default", Count: 1}
	if err := missing.Load(&got); err != nil || got.Name != "default" {
		t.Errorf("missing file: %+v, %v", got, err)
	}

	partial := filepath.Join(dir, "partial.json")
	if err := os.WriteFile(partial, []byte(`{"count": 5}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got = settings{Name: "default", Count: 1}
	if err := NewFile[settings](partial).Load(&got); err != nil || got != (settings{Name: "default", Count: 5}) {
		t.Errorf("partial file: %+v, %v", got, err)
	}
}

func TestCorruptFileIsMovedAside(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "list.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	var got []string
	err := NewFile[[]string](path).Load(&got)
	if !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Load error = %v, want ErrCorrupt", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("corrupt file still in place")
	}
	if aside, _ := filepath.Glob(path + ".broken-*"); len(aside) != 1 {
		t.Errorf("moved-aside files: %v", aside)
	}
}

func TestFileIsPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no Unix permission bits")
	}
	path := filepath.Join(t.TempDir(), "list.json")
	if err := NewFile[[]string](path).Save([]string{"x"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("mode = %o, want 600", mode)
	}
}
