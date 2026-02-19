package hae

import "encoding/json"

// MetricShape describes the data point structure for a metric.
type MetricShape int

const (
	ShapeQty           MetricShape = iota // Standard: {"qty": N}
	ShapeMinAvgMax                        // Heart rate: {"Min": N, "Avg": N, "Max": N}
	ShapeBloodPressure                    // Blood pressure: {"systolic": N, "diastolic": N}
)

// DetectMetricShape returns the expected data point shape for a metric name.
func DetectMetricShape(name string) MetricShape {
	switch name {
	case "heart_rate":
		return ShapeMinAvgMax
	case "blood_pressure":
		return ShapeBloodPressure
	default:
		return ShapeQty
	}
}

// SleepFormat describes whether sleep data is aggregated or per-stage.
type SleepFormat int

const (
	SleepFormatAggregated   SleepFormat = iota // Has "totalSleep" field
	SleepFormatUnaggregated                    // Has "startDate" field
)

// DetectSleepFormat examines a raw JSON data point to determine if it's aggregated or unaggregated.
func DetectSleepFormat(raw json.RawMessage) SleepFormat {
	// Quick probe: unmarshal into a map and check for distinguishing keys
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return SleepFormatAggregated // fallback
	}
	if _, ok := probe["totalSleep"]; ok {
		return SleepFormatAggregated
	}
	if _, ok := probe["startDate"]; ok {
		return SleepFormatUnaggregated
	}
	return SleepFormatAggregated // fallback
}
