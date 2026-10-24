package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFindPrefersSnagTools(t *testing.T) {
	dir := t.TempDir()
	name := "yt-dlp"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SNAG_TOOLS", dir)
	if got := Find("").YtDLP; filepath.Dir(got) != dir {
		t.Errorf("Find found yt-dlp at %q, want the one in $SNAG_TOOLS", got)
	}
}

func TestEverySupportedPlatformHasABuild(t *testing.T) {
	for _, platform := range []string{"linux/amd64", "darwin/arm64", "windows/amd64"} {
		if ytdlpAssets[platform] == "" {
			t.Errorf("no yt-dlp build listed for %s", platform)
		}
	}
}
