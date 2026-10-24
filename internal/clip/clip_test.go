package clip

import (
	"math"
	"testing"
)

func TestRange(t *testing.T) {
	tests := []struct {
		r            Range
		isSet, valid bool
	}{
		{Range{}, false, true},
		{Range{From: 2, To: 5.5}, true, true},
		{Range{From: 0, To: 3}, true, true},
		{Range{From: 5, To: 5}, true, false},
		{Range{From: 8, To: 3}, true, false},
		{Range{From: -1, To: 4}, true, false},
		{Range{From: 1, To: math.Inf(1)}, true, false},
		{Range{From: math.NaN(), To: 4}, true, false},
	}
	for _, tt := range tests {
		if tt.r.IsSet() != tt.isSet || tt.r.Valid() != tt.valid {
			t.Errorf("%+v: IsSet %v, Valid %v; want %v, %v", tt.r, tt.r.IsSet(), tt.r.Valid(), tt.isSet, tt.valid)
		}
	}
}
