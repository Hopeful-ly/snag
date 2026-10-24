package cookies

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var list = []Cookie{
	{Name: "a", Value: "1", Domain: ".example.com", Path: "/", Secure: true, Expires: 1999999999},
	{Name: "b", Value: "2", Domain: "example.com", HostOnly: true},
	{Name: "c", Value: "3", Domain: "other.com"},
}

func TestHeader(t *testing.T) {
	tests := []struct{ address, want string }{
		{"https://cdn.example.com/file", "a=1"},
		{"https://example.com/file", "a=1; b=2"},
		{"https://unrelated.org/", ""},
	}
	for _, tt := range tests {
		u, _ := url.Parse(tt.address)
		if got := Header(list, u); got != tt.want {
			t.Errorf("Header(%s) = %q, want %q", tt.address, got, tt.want)
		}
	}
}

func TestWriteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookies.txt")
	if err := WriteFile(path, list); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "# Netscape HTTP Cookie File\n" +
		".example.com\tTRUE\t/\tTRUE\t1999999999\ta\t1\n" +
		"example.com\tFALSE\t/\tFALSE\t0\tb\t2\n" +
		".other.com\tTRUE\t/\tFALSE\t0\tc\t3\n"
	if string(got) != want {
		t.Errorf("file:\n%s\nwant:\n%s", got, want)
	}
}

func TestTempFile(t *testing.T) {
	path, remove, err := TempFile(list)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || !strings.Contains(string(got), "\ta\t1\n") {
		t.Fatalf("file %q: %q, %v", path, got, err)
	}
	remove()
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Errorf("the file's folder is still there: %v", err)
	}
	if path, remove, err := TempFile(nil); path != "" || err != nil {
		t.Errorf("no cookies gave %q, %v", path, err)
	} else {
		remove()
	}
}
