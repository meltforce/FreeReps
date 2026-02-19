package hae

import (
	"encoding/json"
	"testing"
)

// TestDetectMetricShapeHeartRate verifies that heart_rate is detected as Min/Avg/Max shape.
// Heart rate is the only V1 metric with this shape — wrong detection would lose data.
func TestDetectMetricShapeHeartRate(t *testing.T) {
	if got := DetectMetricShape("heart_rate"); got != ShapeMinAvgMax {
		t.Errorf("heart_rate shape = %d, want ShapeMinAvgMax", got)
	}
}

// TestDetectMetricShapeBloodPressure verifies blood_pressure detection.
func TestDetectMetricShapeBloodPressure(t *testing.T) {
	if got := DetectMetricShape("blood_pressure"); got != ShapeBloodPressure {
		t.Errorf("blood_pressure shape = %d, want ShapeBloodPressure", got)
	}
}

// TestDetectMetricShapeQtyDefault verifies that all other metrics default to qty shape.
func TestDetectMetricShapeQtyDefault(t *testing.T) {
	for _, name := range []string{"resting_heart_rate", "weight_body_mass", "active_energy", "vo2_max"} {
		if got := DetectMetricShape(name); got != ShapeQty {
			t.Errorf("%s shape = %d, want ShapeQty", name, got)
		}
	}
}

// TestDetectSleepFormatAggregated verifies detection of aggregated sleep data.
// Aggregated sleep has "totalSleep" which distinguishes it from per-stage data.
func TestDetectSleepFormatAggregated(t *testing.T) {
	raw := json.RawMessage(`{"date":"2024-02-06","totalSleep":7.5,"core":3.5,"deep":1.5,"rem":2.0}`)
	if got := DetectSleepFormat(raw); got != SleepFormatAggregated {
		t.Errorf("got %d, want SleepFormatAggregated", got)
	}
}

// TestDetectSleepFormatUnaggregated verifies detection of per-stage sleep data.
// Per-stage data has "startDate" which is absent in aggregated data.
func TestDetectSleepFormatUnaggregated(t *testing.T) {
	raw := json.RawMessage(`{"startDate":"2024-02-05 23:00:00 -0800","endDate":"2024-02-05 23:30:00 -0800","value":"Core","qty":0.5}`)
	if got := DetectSleepFormat(raw); got != SleepFormatUnaggregated {
		t.Errorf("got %d, want SleepFormatUnaggregated", got)
	}
}

// TestConvertMetricQty verifies conversion of a standard qty metric data point.
func TestConvertMetricQty(t *testing.T) {
	raw := json.RawMessage(`{"date":"2024-02-06 14:30:00 -0800","qty":58}`)
	row, err := convertMetricDataPoint("resting_heart_rate", "bpm", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row.Qty == nil || *row.Qty != 58 {
		t.Errorf("qty = %v, want 58", row.Qty)
	}
	if row.MetricName != "resting_heart_rate" {
		t.Errorf("name = %q", row.MetricName)
	}
}

// TestConvertMetricMinAvgMax verifies conversion of heart rate (Min/Avg/Max) data.
func TestConvertMetricMinAvgMax(t *testing.T) {
	raw := json.RawMessage(`{"date":"2024-02-06 14:30:00 -0800","Min":65,"Avg":72,"Max":85}`)
	row, err := convertMetricDataPoint("heart_rate", "bpm", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row.MinVal == nil || *row.MinVal != 65 {
		t.Errorf("min = %v, want 65", row.MinVal)
	}
	if row.AvgVal == nil || *row.AvgVal != 72 {
		t.Errorf("avg = %v, want 72", row.AvgVal)
	}
	if row.MaxVal == nil || *row.MaxVal != 85 {
		t.Errorf("max = %v, want 85", row.MaxVal)
	}
	if row.Qty != nil {
		t.Errorf("qty should be nil for heart_rate, got %v", row.Qty)
	}
}

// TestConvertMetricBloodPressure verifies conversion of blood pressure data.
func TestConvertMetricBloodPressure(t *testing.T) {
	raw := json.RawMessage(`{"date":"2024-02-06 14:30:00 -0800","systolic":120,"diastolic":80}`)
	row, err := convertMetricDataPoint("blood_pressure", "mmHg", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row.Systolic == nil || *row.Systolic != 120 {
		t.Errorf("systolic = %v, want 120", row.Systolic)
	}
	if row.Diastolic == nil || *row.Diastolic != 80 {
		t.Errorf("diastolic = %v, want 80", row.Diastolic)
	}
}
