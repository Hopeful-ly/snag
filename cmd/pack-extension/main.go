// Command pack-extension builds the browser extension's release folders and archives.
package main

import (
	"archive/zip"
	"cmp"
	"flag"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/Roham1299/snag/extension"
)

func main() {
	version := flag.String("version", "", "the version to give the Firefox extension (each signed one needs a new one)")
	updateURL := flag.String("update-url", "", "where Firefox looks for newer signed versions")
	flag.Parse()
	dir := cmp.Or(flag.Arg(0), "dist/extension")
	if err := pack(dir, *version, *updateURL); err != nil {
		log.Fatal(err)
	}
}

func pack(dir, version, updateURL string) error {
	chromium, err := extension.Chromium()
	if err != nil {
		return err
	}
	firefox, err := extension.Firefox(version, updateURL)
	if err != nil {
		return err
	}
	name := cmp.Or(version, "dev")
	for _, out := range []struct {
		files  map[string][]byte
		folder string
		zip    string
	}{
		{chromium, "chromium", "snag-extension-" + name + ".zip"},
		{firefox, "firefox", "snag-firefox-" + name + "-unsigned.xpi"},
	} {
		if err := writeFolder(filepath.Join(dir, out.folder), out.files); err != nil {
			return err
		}
		if err := writeZip(filepath.Join(dir, out.zip), out.files); err != nil {
			return err
		}
		fmt.Println(filepath.Join(dir, out.zip))
	}
	return nil
}

func writeFolder(folder string, files map[string][]byte) error {
	if err := os.RemoveAll(folder); err != nil {
		return err
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(folder, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// writeZip uses a fixed file order and timestamp so archives are reproducible.
func writeZip(path string, files map[string][]byte) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	w := zip.NewWriter(f)
	stamp := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, name := range slices.Sorted(maps.Keys(files)) {
		entry, err := w.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: stamp})
		if err != nil {
			return err
		}
		if _, err := entry.Write(files[name]); err != nil {
			return err
		}
	}
	return w.Close()
}
