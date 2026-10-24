package platform

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestChromiumBrowsersAreFoundAndOpenOnTheirExtensionsPage(t *testing.T) {
	bin := t.TempDir()
	called := filepath.Join(t.TempDir(), "called")
	// Stand-ins for the browsers' commands: each writes the address it was opened with.
	for _, name := range []string{"vivaldi-stable", "chromium", "brave", "firefox"} {
		script := "#!/bin/sh\necho \"" + name + " $*\" > " + called + "\n"
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)

	var names []string
	var vivaldi string
	for _, b := range ChromiumBrowsers() {
		names = append(names, b.Name)
		if b.Name == "Vivaldi" {
			vivaldi = b.ID
		}
	}
	if !slices.Equal(names, []string{"Brave", "Chromium", "Vivaldi"}) {
		t.Errorf("found %v, want the Chromium-family ones, by name", names)
	}

	if err := OpenExtensionsPage(vivaldi); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		got, _ := os.ReadFile(called)
		if strings.TrimSpace(string(got)) == "vivaldi-stable vivaldi://extensions" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the browser was started as %q", got)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := OpenExtensionsPage("netscape:/usr/bin/netscape"); err == nil {
		t.Error("opened a browser it doesn't know")
	}
}
