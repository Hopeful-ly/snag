package nativehost

import (
	"os"
	"path/filepath"
)

// Browsers lists the browsers Snag supports on this computer and where they look for helpers.
func Browsers() ([]Browser, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	support := filepath.Join(home, "Library", "Application Support")
	var list []Browser
	for name, dir := range map[string]string{
		"Chrome": "Google/Chrome", "Chromium": "Chromium", "Edge": "Microsoft Edge",
		"Brave": "BraveSoftware/Brave-Browser", "Vivaldi": "Vivaldi", "Opera": "com.operasoftware.Opera",
	} {
		profile := filepath.Join(support, dir)
		list = append(list, Browser{Name: name, Family: Chromium, Profile: profile,
			ManifestDir: filepath.Join(profile, "NativeMessagingHosts")})
	}
	list = append(list, Browser{Name: "Firefox", Family: Firefox, Profile: filepath.Join(support, "Firefox"),
		ManifestDir: filepath.Join(support, "Mozilla", "NativeMessagingHosts")})
	return list, nil
}
