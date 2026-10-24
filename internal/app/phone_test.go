package app

import (
	"regexp"
	"testing"
)

func TestQRPath(t *testing.T) {
	path, size, err := qrPath("http://192.168.1.5:41234/#pair=AbCdEfGhIjKlMnOpQrStUv")
	if err != nil {
		t.Fatal(err)
	}
	if size < 21 || (size-17)%4 != 0 { // 17 + 4×version squares wide
		t.Fatalf("size %d", size)
	}
	// Finder pattern: seven dark squares in each top corner.
	if !regexp.MustCompile(`^M0 0h7v1h-7z`).MatchString(path) {
		t.Fatalf("path starts %.40q", path)
	}
	if !regexp.MustCompile(`^(M\d+ \d+h\d+v1h-\d+z)+$`).MatchString(path) {
		t.Fatal("not only rectangles")
	}
}
