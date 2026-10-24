package platform

import (
	"path/filepath"
	"testing"
)

func TestDefaultPaths(t *testing.T) {
	paths, err := DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for name, p := range map[string]string{
		"Config": paths.Config, "State": paths.State, "Work": paths.Work, "Downloads": paths.Downloads,
		"Socket": paths.Socket, "Extension": paths.Extension, "Tools": paths.Tools,
	} {
		if !filepath.IsAbs(p) {
			t.Errorf("%s = %q, want an absolute path", name, p)
		}
		if seen[p] {
			t.Errorf("%s = %q is used twice", name, p)
		}
		seen[p] = true
	}
}

func TestSnagHomeKeepsEverythingInOneFolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SNAG_HOME", home)
	t.Setenv("SNAG_SOCKET", "")
	paths, err := DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{paths.Config, paths.State, paths.Work, paths.Downloads, paths.Extension} {
		if filepath.Dir(p) != home {
			t.Errorf("%s isn't in $SNAG_HOME", p)
		}
	}
	if real, _ := DefaultPathsWithout(t); paths.Socket == real.Socket {
		t.Errorf("$SNAG_HOME shares the real app's socket %s", paths.Socket)
	}
}

// DefaultPathsWithout returns the paths with no overrides set.
func DefaultPathsWithout(t *testing.T) (Paths, error) {
	t.Setenv("SNAG_HOME", "")
	t.Setenv("SNAG_SOCKET", "")
	return DefaultPaths()
}
