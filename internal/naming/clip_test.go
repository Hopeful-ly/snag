package naming

import (
	"strings"
	"testing"

	"github.com/Roham1299/snag/internal/clip"
)

func TestClipped(t *testing.T) {
	tests := []struct {
		name     string
		from, to float64
		want     string
	}{
		{"Cold Harbor", 42, 78.6, "Cold Harbor (0.42-1.18)"},
		{"Lecture", 3599.5, 3725, "Lecture (59.59-1.02.05)"},
		{"x", 0, 9, "x (0.00-0.09)"},
	}
	for _, tt := range tests {
		if got := Clipped(tt.name, clip.Range{From: tt.from, To: tt.to}); got != tt.want {
			t.Errorf("Clipped(%q, %v, %v) = %q, want %q", tt.name, tt.from, tt.to, got, tt.want)
		}
	}
}

func TestClippedKeepsThePartOnLongNames(t *testing.T) {
	got := Clipped(strings.Repeat("é", 150), clip.Range{From: 1, To: 2})
	if len(got) > maxNameBytes || !strings.HasSuffix(got, " (0.01-0.02)") {
		t.Errorf("Clipped(long) = %q (%d bytes)", got, len(got))
	}
}
