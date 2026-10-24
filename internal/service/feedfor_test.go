package service

import "testing"

func TestFeedFor(t *testing.T) {
	feed := "https://www.youtube.com/feeds/videos.xml?channel_id=UC1"
	for source, want := range map[string]string{
		"https://www.youtube.com/@x/videos":  feed,
		"https://www.youtube.com/@x":         feed,
		"https://www.youtube.com/@x/shorts":  "",
		"https://www.youtube.com/@x/streams": "",
		"https://feeds.example.org/show.rss": feed,
	} {
		if got := feedFor(source, feed); got != want {
			t.Errorf("feedFor(%s) = %q", source, got)
		}
	}
}
