package platform

import (
	"strings"
	"testing"
)

func TestWebAddressKeepsOnlyWhatIsSafeToRecord(t *testing.T) {
	for raw, want := range map[string]string{
		"https://example.com/files/a.bin?token=1":                "https://example.com/files/a.bin?token=1",
		"http://user:secret@example.com/a.bin":                   "http://example.com/a.bin",
		"file:///etc/passwd":                                     "",
		"javascript:alert(1)":                                    "",
		"https://":                                               "",
		"not an address":                                         "",
		"https://example.com/a\r\nHostUrl=b":                     "",
		"https://example.com/" + strings.Repeat("a", maxAddress): "",
	} {
		if got := webAddress(raw); got != want {
			t.Errorf("webAddress(%q) = %q, want %q", raw, got, want)
		}
	}
}
