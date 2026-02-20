package upload

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/claude/freereps/internal/models"
)

func floatPtr(f float64) *float64 { return &f }

// TestConvertMetricQty verifies that standard qty metrics (e.g. weight) are
// correctly converted from .hae Apple-epoch format to REST API date strings.
func TestConvertMetricQty(t *testing.T) {
	file := models.HAEFileMetric{
		Metric: "weight_body_mass",
		Data: []models.HAEFileDataPoint{
			{
				Start: 730000000, // Apple epoch
				Unit:  "kg",
				Qty:   floatPtr(80.5),
			},
		},
	}

	metric, hrPoints, err := convertMetric(file, "weight_body_mass")
	if err != nil {
		t.Fatal(err)
	}

	if len(hrPoints) != 0 {
		t.Errorf("expected 0 hrPoints for weight, got %d", len(hrPoints))
	}

	if metric.Name != "weight_body_mass" {
		t.Errorf("name = %q, want weight_body_mass", metric.Name)
	}
	if metric.Units != "kg" {
		t.Errorf("units = %q, want kg", metric.Units)
	}
	if len(metric.Data) != 1 {
		t.Fatalf("data length = %d, want 1", len(metric.Data))
	}

	// Verify JSON structure
	var point map[string]any
	if err := json.Unmarshal(metric.Data[0], &point); err != nil {
		t.Fatal(err)
	}

	if _, ok := point["date"]; !ok {
		t.Error("missing date field")
	}
	if qty, ok := point["qty"].(float64); !ok || qty != 80.5 {
		t.Errorf("qty = %v, want 80.5", point["qty"])
	}
}

// TestConvertMetricHeartRate verifies that heart_rate data points have their
// min/avg/max fields capitalized (Min/Avg/Max) matching the REST API format,
// and that HR data points are collected for workout correlation.
func TestConvertMetricHeartRate(t *testing.T) {
	file := models.HAEFileMetric{
		Metric: "heart_rate",
		Data: []models.HAEFileDataPoint{
			{
				Start:   730000000,
				Unit:    "count/min",
				Min:     floatPtr(60),
				Avg:     floatPtr(72),
				Max:     floatPtr(85),
				Sources: []models.HAEFileSource{{Name: "Apple Watch"}},
			},
			{
				Start:   730000060,
				Unit:    "count/min",
				Min:     floatPtr(65),
				Avg:     floatPtr(75),
				Max:     floatPtr(90),
				Sources: []models.HAEFileSource{{Name: "Apple Watch"}},
			},
		},
	}

	metric, hrPoints, err := convertMetric(file, "heart_rate")
	if err != nil {
		t.Fatal(err)
	}

	if len(metric.Data) != 2 {
		t.Fatalf("data length = %d, want 2", len(metric.Data))
	}

	// Verify capitalized field names in JSON
	var point map[string]any
	if err := json.Unmarshal(metric.Data[0], &point); err != nil {
		t.Fatal(err)
	}

	if _, ok := point["Min"]; !ok {
		t.Error("missing Min field (should be capitalized)")
	}
	if _, ok := point["Avg"]; !ok {
		t.Error("missing Avg field (should be capitalized)")
	}
	if _, ok := point["Max"]; !ok {
		t.Error("missing Max field (should be capitalized)")
	}

	if point["Min"].(float64) != 60 {
		t.Errorf("Min = %v, want 60", point["Min"])
	}

	// Verify HR data points collected for correlation
	if len(hrPoints) != 2 {
		t.Fatalf("hrPoints length = %d, want 2", len(hrPoints))
	}
	if hrPoints[0].Min != 60 || hrPoints[0].Avg != 72 || hrPoints[0].Max != 85 {
		t.Errorf("hrPoints[0] = %+v, want Min=60 Avg=72 Max=85", hrPoints[0])
	}
	if hrPoints[0].Source != "Apple Watch" {
		t.Errorf("hrPoints[0].Source = %q, want Apple Watch", hrPoints[0].Source)
	}
}

// TestConvertMetricActiveEnergy verifies that active_energy data points with
// non-kcal units (kJ) are filtered out, keeping only kcal values.
func TestConvertMetricActiveEnergy(t *testing.T) {
	file := models.HAEFileMetric{
		Metric: "active_energy",
		Data: []models.HAEFileDataPoint{
			{Start: 730000000, Unit: "kcal", Qty: floatPtr(500)},
			{Start: 730000000, Unit: "kJ", Qty: floatPtr(2092)},
			{Start: 730000060, Unit: "kcal", Qty: floatPtr(100)},
		},
	}

	metric, _, err := convertMetric(file, "active_energy")
	if err != nil {
		t.Fatal(err)
	}

	if len(metric.Data) != 2 {
		t.Errorf("data length = %d, want 2 (kJ should be filtered)", len(metric.Data))
	}
}

// TestConvertSleepStages verifies that .hae sleep data points are converted
// to the REST API's unaggregated format with startDate/endDate/value/qty fields.
func TestConvertSleepStages(t *testing.T) {
	dataPoints := []models.HAEFileDataPoint{
		{Start: 730000000, End: 730003600, Unit: "hr", Deep: floatPtr(1.0)},
		{Start: 730003600, End: 730007200, Unit: "hr", Core: floatPtr(1.0)},
		{Start: 730007200, End: 730010800, Unit: "hr", REM: floatPtr(1.0)},
		{Start: 730010800, End: 730012600, Unit: "hr", Awake: floatPtr(0.5)},
	}

	data, err := convertSleepStages(dataPoints)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) != 4 {
		t.Fatalf("data length = %d, want 4", len(data))
	}

	// Verify the first stage
	var stage models.HAESleepStage
	if err := json.Unmarshal(data[0], &stage); err != nil {
		t.Fatal(err)
	}

	if stage.Value != "Deep" {
		t.Errorf("stage[0].Value = %q, want Deep", stage.Value)
	}
	if stage.Qty != 1.0 {
		t.Errorf("stage[0].Qty = %f, want 1.0", stage.Qty)
	}
	if stage.StartDate.IsZero() || stage.EndDate.IsZero() {
		t.Error("stage dates should not be zero")
	}
}

// TestConvertSleepStagesSkipsEmpty verifies that data points without any
// sleep stage field present are skipped.
func TestConvertSleepStagesSkipsEmpty(t *testing.T) {
	dataPoints := []models.HAEFileDataPoint{
		{Start: 730000000, End: 730003600, Unit: "hr", Qty: floatPtr(5.0)},
	}

	data, err := convertSleepStages(dataPoints)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) != 0 {
		t.Errorf("data length = %d, want 0 (no sleep stages)", len(data))
	}
}

// TestConvertWorkout verifies that .hae workout fields are correctly converted
// to REST API format with HAEQuantity wrappers and embedded route/HR data.
func TestConvertWorkout(t *testing.T) {
	energy := 350.0
	distance := 5.2
	elevation := 120.0

	fileWorkout := models.HAEFileWorkout{
		ID:           "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE",
		Name:         "Running",
		Start:        730000000,
		End:          730003600,
		Duration:     3600,
		ActiveEnergy: &energy,
		TotalDistance: &distance,
		ElevationUp:  &elevation,
		Location:     "Outdoor",
	}

	route := &models.HAEFileRoute{
		ID:   "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE",
		Name: "Running Route",
		Locations: []models.HAEFileLocation{
			{Latitude: 48.1, Longitude: 11.5, Elevation: 500, Speed: 3.5, Course: 90, Time: 730000000, HAcc: 5, VAcc: 3},
			{Latitude: 48.2, Longitude: 11.6, Elevation: 510, Speed: 3.6, Course: 91, Time: 730000060, HAcc: 5, VAcc: 3},
		},
	}

	workout := convertWorkout(fileWorkout, route, nil)

	if workout.ID != "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE" {
		t.Errorf("ID = %q", workout.ID)
	}
	if workout.Name != "Running" {
		t.Errorf("Name = %q", workout.Name)
	}
	if workout.Duration != 3600 {
		t.Errorf("Duration = %f", workout.Duration)
	}
	if workout.Location != "Outdoor" {
		t.Errorf("Location = %q", workout.Location)
	}

	// Verify HAEQuantity wrappers
	if workout.ActiveEnergyBurned == nil || workout.ActiveEnergyBurned.Qty != 350.0 || workout.ActiveEnergyBurned.Units != "kcal" {
		t.Errorf("ActiveEnergyBurned = %+v", workout.ActiveEnergyBurned)
	}
	if workout.Distance == nil || workout.Distance.Qty != 5.2 || workout.Distance.Units != "km" {
		t.Errorf("Distance = %+v", workout.Distance)
	}
	if workout.ElevationUp == nil || workout.ElevationUp.Qty != 120.0 || workout.ElevationUp.Units != "m" {
		t.Errorf("ElevationUp = %+v", workout.ElevationUp)
	}

	// Verify route embedding
	if len(workout.Route) != 2 {
		t.Fatalf("Route length = %d, want 2", len(workout.Route))
	}
	if workout.Route[0].Latitude != 48.1 || workout.Route[0].Longitude != 11.5 {
		t.Errorf("Route[0] = lat=%f lon=%f", workout.Route[0].Latitude, workout.Route[0].Longitude)
	}
	if workout.Route[0].Altitude != 500 {
		t.Errorf("Route[0].Altitude = %f, want 500", workout.Route[0].Altitude)
	}
}

// TestConvertWorkoutOptionalFields verifies that nil optional fields
// remain nil in the converted workout.
func TestConvertWorkoutOptionalFields(t *testing.T) {
	fileWorkout := models.HAEFileWorkout{
		ID:       "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE",
		Name:     "Strength Training",
		Start:    730000000,
		End:      730003600,
		Duration: 3600,
	}

	workout := convertWorkout(fileWorkout, nil, nil)

	if workout.ActiveEnergyBurned != nil {
		t.Error("ActiveEnergyBurned should be nil")
	}
	if workout.Distance != nil {
		t.Error("Distance should be nil")
	}
	if workout.ElevationUp != nil {
		t.Error("ElevationUp should be nil")
	}
	if len(workout.Route) != 0 {
		t.Error("Route should be empty")
	}
	if len(workout.HeartRateData) != 0 {
		t.Error("HeartRateData should be empty")
	}
}

// TestCorrelateWorkoutHR verifies that binary search correctly finds
// heart rate data points within a workout's time range.
func TestCorrelateWorkoutHR(t *testing.T) {
	baseTime := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	hrPoints := make([]hrDataPoint, 100)
	for i := range hrPoints {
		hrPoints[i] = hrDataPoint{
			Time:   baseTime.Add(time.Duration(i) * time.Minute),
			Min:    float64(60 + i),
			Avg:    float64(70 + i),
			Max:    float64(80 + i),
			Units:  "count/min",
			Source: "Apple Watch",
		}
	}

	// Workout from minute 20 to minute 50
	start := baseTime.Add(20 * time.Minute)
	end := baseTime.Add(50 * time.Minute)

	result := correlateWorkoutHR(hrPoints, start, end)

	// Should include points at minutes 20, 21, ..., 50 = 31 points
	if len(result) != 31 {
		t.Errorf("correlated %d HR points, want 31", len(result))
	}

	// Verify first and last point values
	if len(result) > 0 {
		if result[0].Avg != 90 { // 70 + 20
			t.Errorf("first HR point Avg = %f, want 90", result[0].Avg)
		}
		if result[len(result)-1].Avg != 120 { // 70 + 50
			t.Errorf("last HR point Avg = %f, want 120", result[len(result)-1].Avg)
		}
	}
}

// TestCorrelateWorkoutHRNoOverlap verifies that no HR points are returned
// when the workout time range doesn't overlap with any HR data.
func TestCorrelateWorkoutHRNoOverlap(t *testing.T) {
	baseTime := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	hrPoints := []hrDataPoint{
		{Time: baseTime, Avg: 70},
		{Time: baseTime.Add(1 * time.Minute), Avg: 72},
	}

	// Workout is 2 hours later — no overlap
	start := baseTime.Add(2 * time.Hour)
	end := baseTime.Add(3 * time.Hour)

	result := correlateWorkoutHR(hrPoints, start, end)
	if len(result) != 0 {
		t.Errorf("correlated %d HR points, want 0", len(result))
	}
}

// TestConvertWorkoutWithHRCorrelation verifies that HR data is embedded
// in the workout and a summary is computed from the correlated points.
func TestConvertWorkoutWithHRCorrelation(t *testing.T) {
	baseTime := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	appleOffset := float64(baseTime.Unix() - models.AppleEpochOffset)

	hrPoints := []hrDataPoint{
		{Time: baseTime.Add(5 * time.Minute), Min: 80, Avg: 100, Max: 120, Units: "count/min", Source: "Apple Watch"},
		{Time: baseTime.Add(10 * time.Minute), Min: 90, Avg: 130, Max: 160, Units: "count/min", Source: "Apple Watch"},
		{Time: baseTime.Add(15 * time.Minute), Min: 85, Avg: 120, Max: 150, Units: "count/min", Source: "Apple Watch"},
	}

	fileWorkout := models.HAEFileWorkout{
		ID:       "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE",
		Name:     "Running",
		Start:    appleOffset,
		End:      appleOffset + 1800, // 30 minutes
		Duration: 1800,
	}

	workout := convertWorkout(fileWorkout, nil, hrPoints)

	if len(workout.HeartRateData) != 3 {
		t.Fatalf("HeartRateData length = %d, want 3", len(workout.HeartRateData))
	}

	// Verify HR summary was computed
	if workout.HeartRate == nil {
		t.Fatal("HeartRate summary should not be nil")
	}

	expectedAvg := (100.0 + 130.0 + 120.0) / 3.0
	if math.Abs(workout.HeartRate.Avg.Qty-expectedAvg) > 0.01 {
		t.Errorf("HeartRate.Avg = %f, want %f", workout.HeartRate.Avg.Qty, expectedAvg)
	}
	if workout.HeartRate.Min.Qty != 100 {
		t.Errorf("HeartRate.Min = %f, want 100 (min of Avg values)", workout.HeartRate.Min.Qty)
	}
	if workout.HeartRate.Max.Qty != 130 {
		t.Errorf("HeartRate.Max = %f, want 130 (max of Avg values)", workout.HeartRate.Max.Qty)
	}
}

// TestSafeFloat verifies nil pointer handling.
func TestSafeFloat(t *testing.T) {
	if safeFloat(nil) != 0 {
		t.Error("safeFloat(nil) should be 0")
	}
	v := 42.5
	if safeFloat(&v) != 42.5 {
		t.Error("safeFloat(&42.5) should be 42.5")
	}
}

// TestFormatHAETime verifies the time format matches the REST API expectation.
func TestFormatHAETime(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	got := formatHAETime(ts)
	want := "2024-06-15 10:30:00 +0000"
	if got != want {
		t.Errorf("formatHAETime = %q, want %q", got, want)
	}
}
