package nativehost

import (
	"errors"
	"os"
	"path/filepath"
)

// Browsers lists the browsers Snag supports on this computer and where they look for helpers.
// Windows browsers find helpers through the registry; the manifests live with Snag's data.
func Browsers() ([]Browser, error) {
	data := os.Getenv("LOCALAPPDATA")
	if data == "" {
		return nil, errors.New("%LocalAppData% is not set")
	}
	dir := filepath.Join(data, "Snag", "native-messaging")
	var list []Browser
	for name, key := range map[string]string{
		"Chrome": `Software\Google\Chrome`, "Chromium": `Software\Chromium`, "Edge": `Software\Microsoft\Edge`,
		"Brave": `Software\BraveSoftware\Brave-Browser`, "Vivaldi": `Software\Vivaldi`,
	} {
		list = append(list, Browser{Name: name, Family: Chromium, ManifestDir: dir,
			RegistryKey: key + `\NativeMessagingHosts\` + Name})
	}
	list = append(list, Browser{Name: "Firefox", Family: Firefox, ManifestDir: dir,
		RegistryKey: `Software\Mozilla\NativeMessagingHosts\` + Name})
	return list, nil
}
