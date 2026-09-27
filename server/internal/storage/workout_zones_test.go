package storage

import (
	"math"
	"testing"
)

// TestZoneEdgesUseHeartRateReserve pins the edges an easy run on 2026-09-27
// was judged by: with a maximum of 168 and a resting rate of 60, an average of
// 134 bpm sits in zone 2, as the Apple Watch reported, where fractions of the
// maximum put it in zone 3.
func TestZoneEdgesUseHeartRateReserve(t *testing.T) {
	cases := []struct {
		name        string
		maxHR, rest float64
		want        []float64
	}{
		{"reserve", 168, 60, []float64{124.8, 135.6, 146.4, 157.2}},
		{"without a resting rate the maximum alone sets the edges", 168, 0, []float64{100.8, 117.6, 134.4, 151.2}},
		{"a resting rate at or above the maximum is ignored", 168, 170, []float64{100.8, 117.6, 134.4, 151.2}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ZoneEdges(c.maxHR, c.rest)
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if math.Abs(got[i]-c.want[i]) > 1e-9 {
					t.Fatalf("got %v, want %v", got, c.want)
				}
			}
		})
	}

	if got := ZoneEdges(0, 60); got != nil {
		t.Fatalf("no maximum: got %v, want nil", got)
	}
}
