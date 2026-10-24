package streamdl

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestSizeEstimateStaysNearTheTruth(t *testing.T) {
	const fragments, parallel = 1450, 8
	random := rand.New(rand.NewPCG(1, 2))
	sizes := make([]float64, fragments)
	var truth float64
	for i := range sizes {
		sizes[i] = 1.6e6 + random.Float64()*0.8e6
		truth += sizes[i]
	}

	estimate := sizeEstimate{parallel: parallel}
	var shown []int64
	finished, next := 0, 0
	inFlight := map[int]float64{}
	for finished < fragments {
		for len(inFlight) < parallel && next < fragments {
			inFlight[next] = 0
			next++
		}
		for i, got := range inFlight {
			got = math.Min(sizes[i], got+(0.05+random.Float64()*0.35)*sizes[i])
			if got >= sizes[i] {
				delete(inFlight, i)
				finished++
			} else {
				inFlight[i] = got
			}
		}
		var downloaded float64
		for i := range next {
			if got, ok := inFlight[i]; ok {
				downloaded += got
			} else {
				downloaded += sizes[i]
			}
		}
		if total, _ := estimate.update(int64(downloaded), 0, finished, fragments); total > 0 {
			shown = append(shown, total)
		}
	}

	afterStart := shown[len(shown)/50:] // once 2% in
	var worst float64
	changes := 0
	for i, v := range afterStart {
		worst = math.Max(worst, math.Abs(float64(v)-truth)/truth)
		if i > 0 && v != afterStart[i-1] {
			changes++
		}
	}
	if worst > 0.08 || changes > 40 {
		t.Errorf("within %.1f%% of the truth, changed %d times; want within 8%%, under 40 changes", worst*100, changes)
	}
}

func TestSizeEstimate(t *testing.T) {
	exact := sizeEstimate{hint: 500}
	if total, guess := exact.update(10, 1000, 0, 0); total != 1000 || guess {
		t.Errorf("exact size: %d, %v", total, guess)
	}
	hinted := sizeEstimate{hint: 500}
	if total, guess := hinted.update(10, 0, 0, 0); total != 500 || !guess {
		t.Errorf("before any pieces the hint stands in: %d, %v", total, guess)
	}
	unknown := sizeEstimate{}
	if total, _ := unknown.update(10, 0, 0, 0); total != 0 {
		t.Errorf("nothing to go on: %d", total)
	}
}
