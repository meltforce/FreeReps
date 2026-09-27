package storage

import (
	"math"
	"testing"
	"time"
)

// TestAgeYearsCountsCompletedYears exists because an off-by-one here shifts the
// estimated maximum heart rate by a full bpm and, with it, every zone boundary
// on the workouts screen.
func TestAgeYearsCountsCompletedYears(t *testing.T) {
	birth := time.Date(1990, 6, 15, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		now  time.Time
		want int
	}{
		{"the day before the birthday", time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), 35},
		{"on the birthday", time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), 36},
		{"the day after", time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC), 36},
		{"new year, birthday still ahead", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 35},
		{"late in the year", time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), 36},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := AgeYears(birth, tt.now); got != tt.want {
				t.Errorf("AgeYears(%s) = %d, want %d",
					tt.now.Format("2006-01-02"), got, tt.want)
			}
		})
	}
}

// TestAgeYearsAcrossALeapDay guards the YearDay comparison: 29 February shifts
// every later day-of-year by one, so a birthday in March would otherwise count
// a year early in leap years.
func TestAgeYearsAcrossALeapDay(t *testing.T) {
	// 2028 is a leap year; 1 March is day 61 there and day 60 in common years.
	birth := time.Date(1990, 3, 1, 0, 0, 0, 0, time.UTC)

	if got := AgeYears(birth, time.Date(2028, 2, 29, 0, 0, 0, 0, time.UTC)); got != 37 {
		t.Errorf("the day before the birthday in a leap year = %d, want 37", got)
	}
	if got := AgeYears(birth, time.Date(2028, 3, 1, 0, 0, 0, 0, time.UTC)); got != 38 {
		t.Errorf("on the birthday in a leap year = %d, want 38", got)
	}
}

func TestEstimatedMaxHeartRate(t *testing.T) {
	cases := map[int]float64{36: 182.8, 52: 171.6}
	for age, want := range cases {
		if got := EstimatedMaxHeartRate(age); math.Abs(got-want) > 1e-9 {
			t.Errorf("EstimatedMaxHeartRate(%d) = %v, want %v", age, got, want)
		}
	}
}
