package naming

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFromAddress(t *testing.T) {
	tests := []struct {
		address string
		want    string
	}{
		{"https://tunnel5.files.example/KEOeplOG8iaPmzsqlK_up7eLTAMSMtG_jFOEr60voxXL_0SWZPpY8i4KY4tOP3", ""},
		{"https://x.y/dl/ABCDEFGHijklmnopQRSTuvwxYZ12.zip", ""},
		{"https://x.y/a/7a1f3c9e2b.mp4", ""},
		{"https://x.y/vi/1434659607842-pgv4ql-1563742931271.mp4", ""},
		{"https://x.y/d41d8cd9-8f00-b204-e980-0998ecf8427e.mkv", ""},
		{"https://x.y/files/Nivalis-Nights.zip", "Nivalis-Nights.zip"},
		{"https://x.y/ubuntu-26.04-desktop-amd64.iso", "ubuntu-26.04-desktop-amd64.iso"},
		{"https://x.y/reports/report_2025.pdf?token=abc", "report_2025.pdf"},
		{"https://x.y/Holiday%20in%20Lisbon.mp4", "Holiday in Lisbon.mp4"},
		{"https://x.y/video.mp4", ""},
		{"https://x.y/", ""},
		{"https://x.y/folder/", ""},
	}
	for _, tt := range tests {
		if got := FromAddress(tt.address, false); got != tt.want {
			t.Errorf("FromAddress(%q) = %q, want %q", tt.address, got, tt.want)
		}
	}
	if got := FromAddress("https://x.y/video.mp4", true); got != "video.mp4" {
		t.Errorf("generic name allowed: got %q, want video.mp4", got)
	}
}

func TestSafe(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Holiday clip.mp4", "Holiday clip.mp4"},
		{`a/b\c:d*e?f"g<h>i|j.txt`, "a b c d e f g h i j.txt"},
		{"  ..hidden. ", "hidden"},
		{"tabs\tand\nnewlines", "tabs and newlines"},
		{"CON.txt", "CON_.txt"},
		{"nul", "nul_"},
		{"", "fallback"},
		{"...", "fallback"},
	}
	for _, tt := range tests {
		if got := Safe(tt.name, "fallback"); got != tt.want {
			t.Errorf("Safe(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestSafeShortensLongNamesKeepingTheExtension(t *testing.T) {
	long := strings.Repeat("ü", 150) + ".mp4" // 300 bytes of two-byte characters
	got := Safe(long, "")
	if len(got) > maxNameBytes || !strings.HasSuffix(got, ".mp4") || !strings.HasPrefix(got, "üü") {
		t.Errorf("Safe(long) = %q (%d bytes)", got, len(got))
	}
}

func TestFromContentDisposition(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		{`attachment; filename="Holiday clip.mp4"`, "Holiday clip.mp4"},
		{`attachment; filename*=UTF-8''na%C3%AFve%20file.txt`, "naïve file.txt"},
		{`attachment; filename=report 2025.pdf`, "report 2025.pdf"},
		{`inline`, ""},
		{``, ""},
	}
	for _, tt := range tests {
		if got := FromContentDisposition(tt.header); got != tt.want {
			t.Errorf("FromContentDisposition(%q) = %q, want %q", tt.header, got, tt.want)
		}
	}
}

func TestWithExtension(t *testing.T) {
	tests := []struct {
		name, contentType, want string
	}{
		{"clip.mp4", "video/webm", "clip.mp4"},
		{"Holiday clip", "video/mp4", "Holiday clip.mp4"},
		{"song", "audio/mpeg; charset=binary", "song.mp3"},
		{"Mr. Smith goes to Washington", "video/mp4", "Mr. Smith goes to Washington.mp4"},
		{"blob", "application/octet-stream", "blob"},
	}
	for _, tt := range tests {
		if got := WithExtension(tt.name, tt.contentType); got != tt.want {
			t.Errorf("WithExtension(%q, %q) = %q, want %q", tt.name, tt.contentType, got, tt.want)
		}
	}
}

func TestUniquePath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "video.mp4"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	inUse := func(path string) bool { return filepath.Base(path) == "video (1).mp4" }

	got := UniquePath(dir, "video.mp4", inUse)
	if want := filepath.Join(dir, "video (2).mp4"); got != want {
		t.Errorf("UniquePath = %q, want %q", got, want)
	}
	if got := UniquePath(dir, "other.mp4", nil); got != filepath.Join(dir, "other.mp4") {
		t.Errorf("UniquePath for a free name = %q", got)
	}
}
