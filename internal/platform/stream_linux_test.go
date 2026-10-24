package platform

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestStreamArgsFollowTheLaunchersExecLine(t *testing.T) {
	const address = "http://127.0.0.1:4000/s/d1/film.mp4"
	for _, tc := range []struct {
		exec string
		want []string
	}{
		{"haruna %U", []string{"haruna", address}},
		{"mpv --player-operation-mode=pseudo-gui -- %U", []string{"mpv", "--player-operation-mode=pseudo-gui", "--", address}},
		{`"/opt/My Player/player" --name=%c %u`, []string{"/opt/My Player/player", address}},
		{`env "A=say \"hi\"" vlc --started-from-file %U`, []string{"env", `A=say "hi"`, "vlc", "--started-from-file", address}},
		{"player 100%% %u", []string{"player", "100%", address}},
	} {
		if got, ok := streamArgs(tc.exec, address); !ok || !slices.Equal(got, tc.want) {
			t.Errorf("streamArgs(%q) = %q, %v; want %q", tc.exec, got, ok, tc.want)
		}
	}
	for _, filesOnly := range []string{"totem %F", "viewer %f", "%U"} {
		if got, ok := streamArgs(filesOnly, address); ok {
			t.Errorf("streamArgs(%q) = %q; it can't take an address", filesOnly, got)
		}
	}
}

func TestStreamPlayersAreTheVideoPlayersThatTakeAddresses(t *testing.T) {
	home := t.TempDir()
	apps := filepath.Join(home, "data", "applications")
	entry := func(id, body string) {
		t.Helper()
		if err := os.MkdirAll(apps, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(apps, id), []byte("[Desktop Entry]\nType=Application\n"+body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entry("zplayer.desktop", "Name=Z Player\nExec=zplayer %U\nMimeType=video/mp4;video/webm;\n")
	entry("aplayer.desktop", "Name=A Player\nExec=aplayer --url %u\nMimeType=video/mp4;\n")
	entry("browser.desktop", "Name=Browser\nExec=browser %u\nMimeType=text/html;video/webm;x-scheme-handler/http;\n")
	entry("files.desktop", "Name=Files Only\nExec=files %F\nMimeType=video/mp4;\n")
	entry("hidden.desktop", "Name=Hidden\nExec=hidden %U\nMimeType=video/mp4;\nNoDisplay=true\n")
	entry("music.desktop", "Name=Music\nExec=music %U\nMimeType=audio/mpeg;\n")
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_DATA_DIRS", filepath.Join(home, "none"))
	asked := ""
	defaultApp = func(mimeType string) string { asked = mimeType; return "zplayer.desktop" }
	t.Cleanup(func() { defaultApp = xdgDefault })

	got := StreamPlayers("film.mp4")
	want := []Player{{ID: "zplayer.desktop", Name: "Z Player", Default: true}, {ID: "aplayer.desktop", Name: "A Player"}}
	if !slices.Equal(got, want) {
		t.Errorf("StreamPlayers = %+v\nwant %+v (the default first, no browsers, file-only, hidden or music players)", got, want)
	}
	if asked != "video/mp4" {
		t.Errorf("asked for the default %q player, want video/mp4's", asked)
	}
}
