package models

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

// TestAppleTimestampToTime verifies Apple Core Data epoch conversion.
// Apple epoch is 2001-01-01 00:00:00 UTC, so timestamp 0 must equal that date.
func TestAppleTimestampToTime(t *testing.T) {
	got := AppleTimestampToTime(0)
	want := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("AppleTimestampToTime(0) = %v, want %v", got, want)
	}
}

// TestAppleTimestampToTimeReal verifies conversion of a real timestamp from sample data.
// 788223754 Apple seconds = 2025-12-23 23:02:34 UTC.
func TestAppleTimestampToTimeReal(t *testing.T) {
	got := AppleTimestampToTime(788223754)
	want := time.Date(2025, 12, 23, 23, 2, 34, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("AppleTimestampToTime(788223754) = %v, want %v", got, want)
	}
}

// TestAppleTimestampToTimeFractional verifies fractional seconds are preserved.
func TestAppleTimestampToTimeFractional(t *testing.T) {
	got := AppleTimestampToTime(788223754.5)
	if got.Nanosecond() < 499000000 || got.Nanosecond() > 501000000 {
		t.Errorf("fractional part: got %d ns, want ~500000000", got.Nanosecond())
	}
}

// TestHAEFileMetricParseHeartRate verifies parsing a heart_rate .hae file with min/avg/max fields.
func TestHAEFileMetricParseHeartRate(t *testing.T) {
	raw := `{
		"metric": "Heart Rate",
		"date": 788223600,
		"data": [
			{
				"avg": 59, "min": 59, "max": 59,
				"metric": "Heart Rate",
				"start": 788223754, "end": 788223755,
				"unit": "count/min",
				"sources": [{"name": "Apple Watch", "identifier": "com.apple.health.xxx"}]
			}
		]
	}`
	var m HAEFileMetric
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Metric != "Heart Rate" {
		t.Errorf("metric = %q", m.Metric)
	}
	if len(m.Data) != 1 {
		t.Fatalf("data count = %d", len(m.Data))
	}
	dp := m.Data[0]
	if dp.Avg == nil || *dp.Avg != 59 {
		t.Errorf("avg = %v", dp.Avg)
	}
	if dp.Min == nil || *dp.Min != 59 {
		t.Errorf("min = %v", dp.Min)
	}
	if dp.SourceName() != "Apple Watch" {
		t.Errorf("source = %q", dp.SourceName())
	}
}

// TestHAEFileMetricParseQty verifies parsing a standard metric with qty field.
func TestHAEFileMetricParseQty(t *testing.T) {
	raw := `{
		"metric": "Resting Heart Rate",
		"date": 788050800,
		"data": [
			{
				"unit": "count/min",
				"start": 788050814,
				"end": 788137186,
				"qty": 70,
				"metric": "Resting Heart Rate",
				"sources": [{"name": "Watch", "identifier": "xxx"}]
			}
		]
	}`
	var m HAEFileMetric
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	dp := m.Data[0]
	if dp.Qty == nil || *dp.Qty != 70 {
		t.Errorf("qty = %v", dp.Qty)
	}
	if dp.Avg != nil {
		t.Errorf("avg should be nil for qty metric, got %v", dp.Avg)
	}
}

// TestHAEFileDataPointSleepStageType verifies sleep stage detection from field presence.
func TestHAEFileDataPointSleepStageType(t *testing.T) {
	tests := []struct {
		name  string
		dp    HAEFileDataPoint
		want  string
	}{
		{"core", HAEFileDataPoint{Core: ptrFloat(0.5)}, "Core"},
		{"deep", HAEFileDataPoint{Deep: ptrFloat(0.3)}, "Deep"},
		{"rem", HAEFileDataPoint{REM: ptrFloat(0.2)}, "REM"},
		{"awake", HAEFileDataPoint{Awake: ptrFloat(0.1)}, "Awake"},
		{"none", HAEFileDataPoint{}, ""},
		{"zero_core", HAEFileDataPoint{Core: ptrFloat(0)}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dp.SleepStageType()
			if got != tt.want {
				t.Errorf("SleepStageType() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestHAEFileDataPointSleepStageDuration verifies correct duration extraction per stage.
func TestHAEFileDataPointSleepStageDuration(t *testing.T) {
	dp := HAEFileDataPoint{Deep: ptrFloat(0.555)}
	got := dp.SleepStageDuration()
	if math.Abs(got-0.555) > 0.001 {
		t.Errorf("SleepStageDuration() = %f, want 0.555", got)
	}
}

// TestHAEFileWorkoutParse verifies parsing a workout .hae file.
func TestHAEFileWorkoutParse(t *testing.T) {
	raw := `{
		"id": "585BDA5C-5A64-4D5A-A432-6BCA6C7BCDBE",
		"name": "Cycling",
		"start": 787833106.40769,
		"end": 787835103.202923,
		"duration": 1996.795,
		"activeEnergy": 235.264,
		"location": "indoor"
	}`
	var w HAEFileWorkout
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if w.ID != "585BDA5C-5A64-4D5A-A432-6BCA6C7BCDBE" {
		t.Errorf("id = %q", w.ID)
	}
	if w.Name != "Cycling" {
		t.Errorf("name = %q", w.Name)
	}
	if w.ActiveEnergy == nil || math.Abs(*w.ActiveEnergy-235.264) > 0.001 {
		t.Errorf("activeEnergy = %v", w.ActiveEnergy)
	}
	if w.Location != "indoor" {
		t.Errorf("location = %q", w.Location)
	}
	if w.TotalDistance != nil {
		t.Errorf("totalDistance should be nil, got %v", w.TotalDistance)
	}
}

// TestHAEFileWorkoutWithDistance verifies parsing a workout that has distance and elevation.
func TestHAEFileWorkoutWithDistance(t *testing.T) {
	raw := `{
		"id": "D39830A2-4724-4648-8F36-41D7511423B6",
		"name": "Hiking",
		"start": 787321422.438,
		"end": 787324808.576,
		"duration": 3386.138,
		"activeEnergy": 1136.56,
		"totalDistance": 4.768,
		"elevationUp": 25.93,
		"humidity": 90,
		"temperature": 8.235
	}`
	var w HAEFileWorkout
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if w.TotalDistance == nil || math.Abs(*w.TotalDistance-4.768) > 0.001 {
		t.Errorf("totalDistance = %v", w.TotalDistance)
	}
	if w.ElevationUp == nil || math.Abs(*w.ElevationUp-25.93) > 0.01 {
		t.Errorf("elevationUp = %v", w.ElevationUp)
	}
}

// TestHAEFileRouteParse verifies parsing a route .hae file.
func TestHAEFileRouteParse(t *testing.T) {
	raw := `{
		"id": "0EEA1E9E-C117-4BF7-A170-5C0B942CB69A",
		"name": "Hiking",
		"locations": [
			{
				"latitude": 52.634, "longitude": 13.276,
				"elevation": 64.257, "speed": 0.036,
				"course": 285.589, "time": 788182029.022,
				"hAcc": 6.366, "vAcc": 2.055
			}
		]
	}`
	var r HAEFileRoute
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.ID != "0EEA1E9E-C117-4BF7-A170-5C0B942CB69A" {
		t.Errorf("id = %q", r.ID)
	}
	if len(r.Locations) != 1 {
		t.Fatalf("locations count = %d", len(r.Locations))
	}
	loc := r.Locations[0]
	if math.Abs(loc.Latitude-52.634) > 0.001 {
		t.Errorf("latitude = %f", loc.Latitude)
	}
	if math.Abs(loc.Elevation-64.257) > 0.001 {
		t.Errorf("elevation = %f", loc.Elevation)
	}
}

// TestHAEFileDataPointSourceNameEmpty verifies empty source returns empty string.
func TestHAEFileDataPointSourceNameEmpty(t *testing.T) {
	dp := HAEFileDataPoint{}
	if got := dp.SourceName(); got != "" {
		t.Errorf("SourceName() = %q, want empty", got)
	}
}

func ptrFloat(v float64) *float64 {
	return &v
}
