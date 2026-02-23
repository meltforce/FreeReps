package storage

import (
	"math"
	"testing"
)

// TestCircularMeanStdSimple verifies that times near midnight average correctly
// across the 24→0 boundary instead of producing the naive 12:00 result.
func TestCircularMeanStdSimple(t *testing.T) {
	tests := []struct {
		name    string
		hours   []float64
		wantMean float64
		wantStd  bool // just check std > 0
	}{
		{
			name:    "same time",
			hours:   []float64{22.0, 22.0, 22.0},
			wantMean: 22.0,
		},
		{
			name:    "around midnight",
			hours:   []float64{23.0, 1.0},
			wantMean: 0.0,
		},
		{
			name:    "morning cluster",
			hours:   []float64{7.0, 7.5, 8.0},
			wantMean: 7.5,
		},
		{
			name:    "evening cluster",
			hours:   []float64{22.0, 22.5, 23.0},
			wantMean: 22.5,
		},
		{
			name:    "empty",
			hours:   []float64{},
			wantMean: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mean, std := circularMeanStd(tt.hours)
			if math.Abs(mean-tt.wantMean) > 0.1 {
				t.Errorf("circularMeanStd(%v) mean = %.2f, want %.2f", tt.hours, mean, tt.wantMean)
			}
			if len(tt.hours) > 1 {
				// All same → std ≈ 0, different → std > 0
				allSame := true
				for _, h := range tt.hours {
					if h != tt.hours[0] {
						allSame = false
						break
					}
				}
				if allSame && std > 0.01 {
					t.Errorf("expected std ≈ 0 for identical times, got %.4f", std)
				}
				if !allSame && std <= 0 {
					t.Errorf("expected std > 0 for varied times, got %.4f", std)
				}
			}
		})
	}
}

// TestHoursToHHMM verifies the fractional hours → "HH:MM" formatting.
func TestHoursToHHMM(t *testing.T) {
	tests := []struct {
		hours float64
		want  string
	}{
		{0.0, "00:00"},
		{7.5, "07:30"},
		{22.75, "22:45"},
		{23.0, "23:00"},
		{24.0, "00:00"},
		{12.0, "12:00"},
	}

	for _, tt := range tests {
		got := hoursToHHMM(tt.hours)
		if got != tt.want {
			t.Errorf("hoursToHHMM(%.2f) = %q, want %q", tt.hours, got, tt.want)
		}
	}
}

// TestTruncInterval verifies the bucket-to-date_trunc mapping.
func TestTruncInterval(t *testing.T) {
	tests := []struct {
		bucket string
		want   string
	}{
		{"1 week", "week"},
		{"1 month", "month"},
		{"anything else", "month"},
	}

	for _, tt := range tests {
		got := truncInterval(tt.bucket)
		if got != tt.want {
			t.Errorf("truncInterval(%q) = %q, want %q", tt.bucket, got, tt.want)
		}
	}
}
