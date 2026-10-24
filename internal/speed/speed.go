// Package speed measures how fast a download goes.
package speed

import "time"

// samples is how many recent measurements the speed is the average of.
const samples = 6

// minGap is the least time between measurements: closer ones (lines a program printed at once)
// would make the speed jump.
const minGap = 200 * time.Millisecond

// Meter averages a download's speed over its last few measurements. The zero Meter is ready.
type Meter struct {
	lastDone int64
	lastTime time.Time
	rates    []float64
}

// Sample takes how many bytes are done at now, and returns the speed in bytes a second.
func (m *Meter) Sample(done int64, now time.Time) int64 {
	if !m.lastTime.IsZero() {
		gap := now.Sub(m.lastTime)
		if gap < minGap {
			return m.average()
		}
		m.rates = append(m.rates, float64(done-m.lastDone)/gap.Seconds())
		if len(m.rates) > samples {
			m.rates = m.rates[1:]
		}
	}
	m.lastDone, m.lastTime = done, now
	return m.average()
}

func (m *Meter) average() int64 {
	if len(m.rates) == 0 {
		return 0
	}
	var sum float64
	for _, r := range m.rates {
		sum += r
	}
	return max(int64(sum/float64(len(m.rates))), 0)
}
