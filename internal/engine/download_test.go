package engine

import "testing"

func TestName(t *testing.T) {
	tests := []struct {
		d    Download
		want string
	}{
		{Download{Path: "/dl/Clip (1).mp4", Filename: "Clip.mp4"}, "Clip (1).mp4"},
		{Download{URL: "https://x/a/token", Filename: "Chosen.zip"}, "Chosen.zip"},
		{Download{URL: "https://x/files/Nivalis-Nights.zip", Title: "Page"}, "Nivalis-Nights.zip"},
		{Download{URL: "https://x/KEOeplOG8iaPmzsqlK_up7eLTAMSMtG", Title: "A Film"}, "A Film"},
		{Download{URL: "https://x/get/video.mp4"}, "video.mp4"},
		{Download{URL: "https://x/"}, "https://x/"},
	}
	for _, tt := range tests {
		if got := tt.d.Name(); got != tt.want {
			t.Errorf("Name(%+v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
