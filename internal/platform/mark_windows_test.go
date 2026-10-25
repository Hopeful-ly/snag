package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMarkDownloadedWritesTheMarkOfTheWeb(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.bin")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MarkDownloaded(path, "https://cdn.example.com/a.bin?e=1", "https://example.com/page"); err != nil {
		t.Fatal(err)
	}
	mark, err := os.ReadFile(path + ":Zone.Identifier")
	if err != nil {
		t.Fatal(err)
	}
	want := "[ZoneTransfer]\r\nZoneId=3\r\nReferrerUrl=https://example.com/page\r\nHostUrl=https://cdn.example.com/a.bin?e=1\r\n"
	if string(mark) != want {
		t.Errorf("Zone.Identifier = %q, want %q", mark, want)
	}
	if data, _ := os.ReadFile(path); string(data) != "data" {
		t.Errorf("the file itself = %q", data)
	}
}

func TestMarkDownloadedWithoutAddresses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.bin")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MarkDownloaded(path, "", ""); err != nil {
		t.Fatal(err)
	}
	mark, _ := os.ReadFile(path + ":Zone.Identifier")
	if want := "[ZoneTransfer]\r\nZoneId=3\r\n"; string(mark) != want {
		t.Errorf("Zone.Identifier = %q, want %q", mark, want)
	}
}
