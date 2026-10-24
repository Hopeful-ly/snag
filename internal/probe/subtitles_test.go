package probe

import (
	"slices"
	"testing"

	"github.com/Roham1299/snag/internal/ytdlp"
)

func TestSubtitlesOfKeepsUploaderTracksAndAFewAutos(t *testing.T) {
	info := ytdlp.Info{
		Language: "ja",
		Subtitles: map[string][]ytdlp.Caption{
			"ja": {{Ext: "vtt", Name: "Japanese"}},
			"en": {{Ext: "vtt", Name: "English"}},
		},
		AutomaticCaptions: map[string][]ytdlp.Caption{
			"ja": {{Ext: "vtt", Name: "Japanese"}},
			"en": {{Ext: "vtt", Name: "English"}},
			"de": {{Ext: "vtt", Name: "German"}},
			"fr": {{Ext: "vtt", Name: "French"}},
			"es": {{Ext: "vtt", Name: "Spanish"}},
		},
	}
	got := subtitlesOf(info)
	var codes []string
	for _, s := range got {
		codes = append(codes, s.Code)
		if s.Auto {
			t.Errorf("%s was marked automatic though an uploader track exists", s.Code)
		}
	}
	if !slices.Equal(codes, []string{"en", "ja"}) {
		t.Fatalf("codes = %v, want English then Japanese (no auto translations)", codes)
	}
}

func TestSubtitlesOfOffersOriginalAndEnglishAutosWhenThereAreNoUploaderTracks(t *testing.T) {
	info := ytdlp.Info{
		Language: "ja",
		AutomaticCaptions: map[string][]ytdlp.Caption{
			"ja-orig": {{Ext: "vtt", Name: "Japanese (auto-generated)"}},
			"en":      {{Ext: "vtt"}},
			"de":      {{Ext: "vtt", Name: "German"}},
			"fr":      {{Ext: "vtt", Name: "French"}},
		},
	}
	got := subtitlesOf(info)
	if len(got) != 2 {
		t.Fatalf("got %+v, want original + English", got)
	}
	if got[0].Code != "en" || !got[0].Auto || got[0].Name != "English" {
		t.Errorf("English auto = %+v", got[0])
	}
	if got[1].Code != "ja-orig" || !got[1].Auto || got[1].Name != "Japanese" {
		t.Errorf("original auto = %+v", got[1])
	}
}

func TestSubtitlesOfReadsHLSSubtitleFormats(t *testing.T) {
	info := ytdlp.Info{Formats: []ytdlp.Format{
		{FormatID: "hls-1080", Ext: "mp4", Height: 1080, VCodec: "avc1"},
		{FormatID: "en-0", Ext: "vtt", VCodec: "none", ACodec: "none", Language: "en", URL: "https://x/en.m3u8"},
		{FormatID: "sub-es", Ext: "vtt", VCodec: "none", ACodec: "none", URL: "https://x/es.m3u8"},
	}}
	got := subtitlesOf(info)
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	if got[0].Code != "en" || got[0].Name != "English" {
		t.Errorf("English = %+v", got[0])
	}
	if got[1].Code != "es" || got[1].Name != "Español" {
		t.Errorf("Spanish = %+v", got[1])
	}
}

func TestSubtitlesOfSkipsLiveChat(t *testing.T) {
	info := ytdlp.Info{Subtitles: map[string][]ytdlp.Caption{
		"live_chat": {{Ext: "json"}},
		"en":        {{Ext: "vtt", Name: "English"}},
	}}
	got := subtitlesOf(info)
	if len(got) != 1 || got[0].Code != "en" {
		t.Errorf("got %+v", got)
	}
}
