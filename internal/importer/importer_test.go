package importer

import (
	"encoding/json"
	"testing"

	"github.com/claude/freereps/internal/models"
)

// TestParseWorkoutUUID extracts UUID from the end of a workout filename.
// The type portion may contain underscores (e.g. "traditional_strength_training").
func TestParseWorkoutUUID(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantUUID string
		wantErr  bool
	}{
		{
			name:     "simple type",
			filename: "cycling_20251219_585BDA5C-5A64-4D5A-A432-6BCA6C7BCDBE.hae",
			wantUUID: "585BDA5C-5A64-4D5A-A432-6BCA6C7BCDBE",
		},
		{
			name:     "multi-word type",
			filename: "traditional_strength_training_20260108_FB20EEB3-B2F8-414F-822E-F3080E24F164.hae",
			wantUUID: "FB20EEB3-B2F8-414F-822E-F3080E24F164",
		},
		{
			name:     "hiit type",
			filename: "high_intensity_interval_training_20251213_223921F7-DE57-4555-8240-F312174952B2.hae",
			wantUUID: "223921F7-DE57-4555-8240-F312174952B2",
		},
		{
			name:     "too short",
			filename: "short.hae",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseWorkoutUUID(tt.filename)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got UUID %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantUUID {
				t.Errorf("got %q, want %q", got, tt.wantUUID)
			}
		})
	}
}

// TestHealthMetricConversion verifies that a standard .hae health metric data point
// is correctly parsed and converted to the expected types.
func TestHealthMetricConversion(t *testing.T) {
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
				"sources": [{"name": "Apple Watch", "identifier": "xxx"}]
			}
		]
	}`
	var file models.HAEFileMetric
	if err := json.Unmarshal([]byte(raw), &file); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(file.Data) != 1 {
		t.Fatalf("data count = %d", len(file.Data))
	}

	dp := file.Data[0]
	ts := models.AppleTimestampToTime(dp.Start)

	// Verify the timestamp is reasonable (year 2025+)
	if ts.Year() < 2025 {
		t.Errorf("converted time year = %d, expected >= 2025", ts.Year())
	}

	if dp.Qty == nil || *dp.Qty != 70 {
		t.Errorf("qty = %v, want 70", dp.Qty)
	}
	if dp.SourceName() != "Apple Watch" {
		t.Errorf("source = %q, want Apple Watch", dp.SourceName())
	}
}

// TestActiveEnergyDualUnit verifies that active energy files contain both kJ and kcal
// entries and that we can filter by unit.
func TestActiveEnergyDualUnit(t *testing.T) {
	raw := `{
		"date": 788742000,
		"metric": "Active Energy",
		"data": [
			{"unit": "kJ", "qty": 0.012, "start": 788742341, "end": 788742342, "metric": "Active Energy", "sources": []},
			{"unit": "kcal", "qty": 0.002, "start": 788742341, "end": 788742342, "metric": "Active Energy", "sources": []},
			{"unit": "kJ", "qty": 0.023, "start": 788742342, "end": 788742343, "metric": "Active Energy", "sources": []},
			{"unit": "kcal", "qty": 0.005, "start": 788742342, "end": 788742343, "metric": "Active Energy", "sources": []}
		]
	}`
	var file models.HAEFileMetric
	if err := json.Unmarshal([]byte(raw), &file); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Count kcal entries only (what the importer should use)
	var kcalCount int
	for _, dp := range file.Data {
		if dp.Unit == "kcal" {
			kcalCount++
		}
	}
	if kcalCount != 2 {
		t.Errorf("kcal entries = %d, want 2", kcalCount)
	}
}

// TestSleepStageDetection verifies that sleep stage type can be detected from
// the .hae sleep_analysis data format where the stage type is a field name.
func TestSleepStageDetection(t *testing.T) {
	raw := `{
		"metric": "Sleep Analysis",
		"data": [
			{
				"start": 788135607.886, "end": 788137608.019,
				"unit": "hr", "totalSleep": 0.555,
				"core": 0.555, "metric": "Sleep Analysis",
				"sources": [{"name": "Watch", "identifier": "xxx"}], "meta": {}
			},
			{
				"start": 788137608.019, "end": 788137846.841,
				"unit": "hr", "totalSleep": 0.066,
				"deep": 0.066, "metric": "Sleep Analysis",
				"sources": [{"name": "Watch", "identifier": "xxx"}], "meta": {}
			},
			{
				"start": 788137846.841, "end": 788137906.546,
				"unit": "hr", "totalSleep": 0.033,
				"awake": 0.033, "metric": "Sleep Analysis",
				"sources": [{"name": "Watch", "identifier": "xxx"}], "meta": {}
			}
		]
	}`
	var file models.HAEFileMetric
	if err := json.Unmarshal([]byte(raw), &file); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	expectedStages := []string{"Core", "Deep", "Awake"}
	for i, dp := range file.Data {
		got := dp.SleepStageType()
		if got != expectedStages[i] {
			t.Errorf("data[%d] stage = %q, want %q", i, got, expectedStages[i])
		}
	}
}

// TestWorkoutFileConversion verifies that a .hae workout JSON can be parsed
// and its fields correctly extracted.
func TestWorkoutFileConversion(t *testing.T) {
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
	var w models.HAEFileWorkout
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	startTime := models.AppleTimestampToTime(w.Start)
	endTime := models.AppleTimestampToTime(w.End)

	if startTime.Year() < 2025 {
		t.Errorf("start year = %d, expected >= 2025", startTime.Year())
	}
	if !endTime.After(startTime) {
		t.Errorf("end time %v should be after start time %v", endTime, startTime)
	}
	if w.Name != "Hiking" {
		t.Errorf("name = %q", w.Name)
	}
}

// TestRouteLocationConversion verifies route location timestamps are correctly converted.
func TestRouteLocationConversion(t *testing.T) {
	raw := `{
		"id": "0EEA1E9E-C117-4BF7-A170-5C0B942CB69A",
		"name": "Hiking",
		"locations": [
			{
				"latitude": 52.634, "longitude": 13.276,
				"elevation": 64.257, "speed": 0.036,
				"course": 285.589, "time": 788182029.022,
				"hAcc": 6.366, "vAcc": 2.055
			},
			{
				"latitude": 52.635, "longitude": 13.277,
				"elevation": 63.397, "speed": 0.162,
				"course": 290.261, "time": 788182030.022,
				"hAcc": 5.500, "vAcc": 1.749
			}
		]
	}`
	var route models.HAEFileRoute
	if err := json.Unmarshal([]byte(raw), &route); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(route.Locations) != 2 {
		t.Fatalf("locations = %d, want 2", len(route.Locations))
	}

	t1 := models.AppleTimestampToTime(route.Locations[0].Time)
	t2 := models.AppleTimestampToTime(route.Locations[1].Time)

	if !t2.After(t1) {
		t.Errorf("second location time %v should be after first %v", t2, t1)
	}
}
