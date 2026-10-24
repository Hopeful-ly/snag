package nativehost

import (
	"cmp"
	"os"
	"path/filepath"
)

// Browsers lists the browsers Snag supports on this computer and where they look for helpers.
func Browsers() ([]Browser, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	config := cmp.Or(os.Getenv("XDG_CONFIG_HOME"), filepath.Join(home, ".config"))
	var list []Browser
	for name, dir := range map[string]string{
		"Chrome": "google-chrome", "Chromium": "chromium", "Edge": "microsoft-edge",
		"Brave": "BraveSoftware/Brave-Browser", "Vivaldi": "vivaldi", "Opera": "opera",
	} {
		profile := filepath.Join(config, dir)
		list = append(list, Browser{Name: name, Family: Chromium, Profile: profile,
			ManifestDir: filepath.Join(profile, "NativeMessagingHosts")})
	}
	// Newer Firefox keeps its profiles under ~/.config/mozilla but still reads helpers from ~/.mozilla.
	firefox := filepath.Join(home, ".mozilla", "firefox")
	if _, err := os.Stat(filepath.Join(config, "mozilla", "firefox")); err == nil {
		firefox = filepath.Join(config, "mozilla", "firefox")
	}
	list = append(list, Browser{Name: "Firefox", Family: Firefox, Profile: firefox,
		ManifestDir: filepath.Join(home, ".mozilla", "native-messaging-hosts")})
	return list, nil
}
