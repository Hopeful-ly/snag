package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallers(t *testing.T) {
	sums := filepath.Join(t.TempDir(), "SHA256SUMS")
	data := "" +
		"aa00000000000000000000000000000000000000000000000000000000000000  snag_1.2.0_amd64.deb\n" +
		"BB00000000000000000000000000000000000000000000000000000000000000 *Snag-1.2.0-windows-amd64-setup.exe\n" +
		"cc00000000000000000000000000000000000000000000000000000000000000  snag-firefox-1.2.0.xpi\n"
	if err := os.WriteFile(sums, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := installers(sums, "https://example.com/v1.2.0/")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("installers %+v, want the .deb and the setup", files)
	}
	if f := files["deb-amd64"]; f.URL != "https://example.com/v1.2.0/snag_1.2.0_amd64.deb" || f.SHA256[:2] != "aa" {
		t.Errorf("deb %+v", f)
	}
	if f := files["windows-amd64"]; f.URL != "https://example.com/v1.2.0/Snag-1.2.0-windows-amd64-setup.exe" || f.SHA256[:2] != "bb" {
		t.Errorf("setup %+v", f)
	}

	if err := os.WriteFile(sums, []byte(data[len(data)-89:]), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := installers(sums, "https://example.com/"); err == nil {
		t.Error("took sums without installers")
	}
}
