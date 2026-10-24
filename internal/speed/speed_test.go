package speed

import (
	"testing"
	"time"
)

func TestMeterAveragesRecentSamples(t *testing.T) {
	var m Meter
	start := time.Now()
	if got := m.Sample(0, start); got != 0 {
		t.Fatalf("first sample: %d, want 0", got)
	}
	if got := m.Sample(1000, start.Add(time.Second)); got != 1000 {
		t.Errorf("after 1000 B in 1 s: %d", got)
	}
	if got := m.Sample(4000, start.Add(2*time.Second)); got != 2000 {
		t.Errorf("after 1000 then 3000 B/s: %d, want their average", got)
	}
	// Two lines printed at once mustn't count as a burst of speed.
	if got := m.Sample(9000, start.Add(2*time.Second+time.Millisecond)); got != 2000 {
		t.Errorf("a sample a moment later changed the speed to %d", got)
	}
}
