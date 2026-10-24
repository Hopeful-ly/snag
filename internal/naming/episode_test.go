package naming

import "testing"

func TestParseEpisode(t *testing.T) {
	tests := []struct {
		in     string
		series string
		s, e   int
	}{
		{"Severance S02E03 - Cold Harbor", "Severance", 2, 3},
		{"Severance S02E03 - Cold Harbor.mp4", "Severance", 2, 3},
		{"Lanterns S1E3 - In Brightest Day", "Lanterns", 1, 3},
		{"Lanterns - S01E03 - The Ring | A", "Lanterns", 1, 3},
		{"Lanterns 1x03", "Lanterns", 1, 3},
		{"Season 2 Episode 11 of Lanterns", "", 0, 0}, // no series before the numbers
		{"The Movie (2024)", "", 0, 0},
		{"Nivalis-Nights.zip", "", 0, 0},
	}
	for _, tt := range tests {
		got, ok := ParseEpisode(tt.in)
		if tt.s == 0 {
			if ok {
				t.Errorf("ParseEpisode(%q) = %+v, want none", tt.in, got)
			}
			continue
		}
		if !ok || got.Series != tt.series || got.Season != tt.s || got.Episode != tt.e {
			t.Errorf("ParseEpisode(%q) = %+v, %v; want %s S%02dE%02d", tt.in, got, ok, tt.series, tt.s, tt.e)
		}
	}
}

func TestSameEpisode(t *testing.T) {
	if !SameEpisode("Severance S02E03 - Cold Harbor", "Severance S02E03 - Cold Harbor.mp4") {
		t.Error("same episode with a file extension")
	}
	if !SameEpisode("Severance S2E3", "severance S02E03 - other title") {
		t.Error("series spelling and padding")
	}
	if SameEpisode("Severance S02E03", "Severance S02E04") {
		t.Error("different episodes")
	}
	if SameEpisode("Severance S02E03", "The Bear S02E03") {
		t.Error("different series")
	}
}
