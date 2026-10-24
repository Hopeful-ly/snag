package nativehost

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/Roham1299/snag/extension"
)

func TestRegisterAndUnregister(t *testing.T) {
	root := t.TempDir()
	installed := filepath.Join(root, "vivaldi")
	if err := os.MkdirAll(installed, 0o755); err != nil {
		t.Fatal(err)
	}
	browsers := []Browser{
		{Name: "Vivaldi", Family: Chromium, Profile: installed, ManifestDir: filepath.Join(installed, "NativeMessagingHosts")},
		{Name: "Chrome", Family: Chromium, Profile: filepath.Join(root, "missing"), ManifestDir: filepath.Join(root, "missing", "hosts")},
		{Name: "Firefox", Family: Firefox, ManifestDir: filepath.Join(root, "mozilla")},
	}
	if runtime.GOOS == "windows" {
		for i := range browsers {
			browsers[i].RegistryKey = fmt.Sprintf(`Software\SnagTest%d\%s\%s`, os.Getpid(), browsers[i].Name, Name)
		}
	}
	host := filepath.Join(root, "bin", "snag-host")

	done, err := Register(host, browsers)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(done, []string{"Vivaldi", "Firefox"}) {
		t.Errorf("registered with %v, want the installed browsers", done)
	}

	var chromium, firefox map[string]any
	readJSON(t, browsers[0].manifestPath(), &chromium)
	readJSON(t, browsers[2].manifestPath(), &firefox)
	if chromium["path"] != host || chromium["name"] != Name || chromium["type"] != "stdio" ||
		fmt.Sprint(chromium["allowed_origins"]) != "["+chromiumOrigin+"]" {
		t.Errorf("Chromium manifest = %v", chromium)
	}
	if fmt.Sprint(firefox["allowed_extensions"]) != "["+firefoxID+"]" {
		t.Errorf("Firefox manifest = %v", firefox)
	}
	if _, err := os.Stat(browsers[1].ManifestDir); !os.IsNotExist(err) {
		t.Error("registered with a browser that isn't installed")
	}

	if err := Unregister(browsers); err != nil {
		t.Fatal(err)
	}
	for _, b := range browsers {
		if _, err := os.Stat(b.manifestPath()); !os.IsNotExist(err) {
			t.Errorf("%s's manifest is still there", b.Name)
		}
	}
}

func TestBrowsersForThisOS(t *testing.T) {
	browsers, err := Browsers()
	if err != nil {
		t.Fatal(err)
	}
	families := map[Family]int{}
	for _, b := range browsers {
		families[b.Family]++
		if !filepath.IsAbs(b.ManifestDir) {
			t.Errorf("%s: manifest folder %q isn't absolute", b.Name, b.ManifestDir)
		}
	}
	if families[Chromium] < 4 || families[Firefox] != 1 {
		t.Errorf("browsers by family: %v", families)
	}
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatal(err)
	}
}

// The Firefox id is written twice, in the extension's manifest and here; the browser only lets
// the extension talk to the helper if they agree.
func TestFirefoxIDMatchesTheExtension(t *testing.T) {
	if id, err := extension.FirefoxID(); err != nil || id != firefoxID {
		t.Errorf("the extension's id is %q (%v), the helper allows %q", id, err, firefoxID)
	}
}
