package models

import (
	"encoding/json"
	"testing"
	"time"
)

// TestParseHealthTimeFullDatetime verifies parsing the standard health datetime format.
// This is the most common format used by all metric data points.
func TestParseHealthTimeFullDatetime(t *testing.T) {
	got, err := ParseHealthTime("2024-02-06 14:30:00 -0800")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2024, 2, 6, 14, 30, 0, 0, time.FixedZone("", -8*3600))
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestParseHealthTimeDateOnly verifies parsing the date-only format used in aggregated sleep data.
// The "date" field in SleepAggregated uses "2024-02-06" without time/timezone.
func TestParseHealthTimeDateOnly(t *testing.T) {
	got, err := ParseHealthTime("2024-02-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Year() != 2024 || got.Month() != 2 || got.Day() != 6 {
		t.Errorf("got %v, want 2024-02-06", got)
	}
}

// TestParseHealthTimeInvalid verifies that an invalid date string returns an error.
// Prevents silent data corruption from malformed timestamps.
func TestParseHealthTimeInvalid(t *testing.T) {
	_, err := ParseHealthTime("not-a-date")
	if err == nil {
		t.Fatal("expected error for invalid date")
	}
}

// TestHealthTimeUnmarshalJSON verifies that HealthTime correctly deserializes from JSON.
// Ensures the custom unmarshaler integrates with encoding/json.
func TestHealthTimeUnmarshalJSON(t *testing.T) {
	var dp HealthMetricDataPoint
	raw := `{"date": "2024-02-06 14:30:00 -0800", "qty": 72.5}`
	if err := json.Unmarshal([]byte(raw), &dp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if dp.Qty != 72.5 {
		t.Errorf("qty = %f, want 72.5", dp.Qty)
	}
	if dp.Date.Year() != 2024 {
		t.Errorf("year = %d, want 2024", dp.Date.Year())
	}
}

// TestHealthPayloadUnmarshal verifies parsing a complete health REST API payload.
// Ensures the nested data.metrics structure is correctly deserialized.
func TestHealthPayloadUnmarshal(t *testing.T) {
	raw := `{
		"data": {
			"metrics": [
				{
					"name": "heart_rate",
					"units": "bpm",
					"data": [
						{"date": "2024-02-06 14:30:00 -0800", "Min": 65, "Avg": 72, "Max": 85}
					]
				}
			],
			"workouts": []
		}
	}`
	var p HealthPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(p.Data.Metrics) != 1 {
		t.Fatalf("metrics count = %d, want 1", len(p.Data.Metrics))
	}
	if p.Data.Metrics[0].Name != "heart_rate" {
		t.Errorf("name = %q, want %q", p.Data.Metrics[0].Name, "heart_rate")
	}
	if len(p.Data.Metrics[0].Data) != 1 {
		t.Fatalf("data points = %d, want 1", len(p.Data.Metrics[0].Data))
	}

	// Parse the raw data point as HeartRate type
	var hr HeartRateDataPoint
	if err := json.Unmarshal(p.Data.Metrics[0].Data[0], &hr); err != nil {
		t.Fatalf("unmarshal hr: %v", err)
	}
	if hr.Avg != 72 {
		t.Errorf("avg = %f, want 72", hr.Avg)
	}
}

// TestHealthWorkoutUnmarshal verifies parsing a Version 2 workout with nested quantity objects.
// Workouts have a different structure than metrics — units are inline objects.
func TestHealthWorkoutUnmarshal(t *testing.T) {
	raw := `{
		"id": "550e8400-e29b-41d4-a716-446655440000",
		"name": "Running",
		"start": "2024-02-06 07:00:00 -0800",
		"end": "2024-02-06 07:30:00 -0800",
		"duration": 1800,
		"activeEnergyBurned": {"qty": 350, "units": "kcal"},
		"distance": {"qty": 3.5, "units": "mi"},
		"heartRateData": [
			{"date": "2024-02-06 07:00:00 -0800", "Min": 120, "Avg": 150, "Max": 175, "units": "bpm"}
		],
		"route": [
			{"latitude": 37.7749, "longitude": -122.4194, "altitude": 50.5, "timestamp": "2024-02-06 07:00:00 -0800", "speed": 7.0}
		]
	}`
	var w HealthWorkout
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if w.ID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("id = %q", w.ID)
	}
	if w.Name != "Running" {
		t.Errorf("name = %q", w.Name)
	}
	if w.Duration != 1800 {
		t.Errorf("duration = %f, want 1800", w.Duration)
	}
	if w.ActiveEnergyBurned == nil || w.ActiveEnergyBurned.Qty != 350 {
		t.Errorf("activeEnergyBurned = %v", w.ActiveEnergyBurned)
	}
	if w.Distance == nil || w.Distance.Qty != 3.5 {
		t.Errorf("distance = %v", w.Distance)
	}
	if len(w.HeartRateData) != 1 || w.HeartRateData[0].Avg != 150 {
		t.Errorf("heartRateData = %v", w.HeartRateData)
	}
	if len(w.Route) != 1 || w.Route[0].Latitude != 37.7749 {
		t.Errorf("route = %v", w.Route)
	}
}
