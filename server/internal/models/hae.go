package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// HAETime handles the Health Auto Export date format: "2006-01-02 15:04:05 -0700"
// Also handles date-only format "2006-01-02" used in aggregated sleep data.
type HAETime struct {
	time.Time
}

const (
	HAETimeLayout     = "2006-01-02 15:04:05 -0700"
	HAEDateOnlyLayout = "2006-01-02"
)

func (t *HAETime) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	return t.Parse(s)
}

func (t HAETime) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Format(HAETimeLayout))
}

// Parse parses a HAE time string, trying full datetime first, then date-only.
func (t *HAETime) Parse(s string) error {
	parsed, err := time.Parse(HAETimeLayout, s)
	if err == nil {
		t.Time = parsed
		return nil
	}
	parsed, err2 := time.Parse(HAEDateOnlyLayout, s)
	if err2 == nil {
		t.Time = parsed
		return nil
	}
	return fmt.Errorf("cannot parse HAE time %q: %w", s, err)
}

// ParseHAETime parses a HAE time string into a time.Time.
func ParseHAETime(s string) (time.Time, error) {
	var t HAETime
	if err := t.Parse(s); err != nil {
		return time.Time{}, err
	}
	return t.Time, nil
}

// HAEPayload is the top-level REST API JSON structure.
type HAEPayload struct {
	Data HAEData `json:"data"`
}

// HAEData contains the arrays of health data.
type HAEData struct {
	Metrics             []HAEMetric             `json:"metrics"`
	Workouts            []HAEWorkout            `json:"workouts"`
	ECGRecordings       []HAEECGRecording       `json:"ecg_recordings,omitempty"`
	Audiograms          []HAEAudiogram          `json:"audiograms,omitempty"`
	ActivitySummaries   []HAEActivitySummary    `json:"activity_summaries,omitempty"`
	Medications         []HAEMedication         `json:"medications,omitempty"`
	VisionPrescriptions []HAEVisionPrescription `json:"vision_prescriptions,omitempty"`
	StateOfMind         []HAEStateOfMind        `json:"state_of_mind,omitempty"`
	CategorySamples     []HAECategorySample     `json:"category_samples,omitempty"`
}

// HAEMetric is a single metric entry with name, units, and data points.
type HAEMetric struct {
	Name  string            `json:"name"`
	Units string            `json:"units"`
	Data  []json.RawMessage `json:"data"`
}

// HAEMetricDataPoint is a standard metric data point with qty.
type HAEMetricDataPoint struct {
	Date       HAETime `json:"date"`
	Qty        float64 `json:"qty"`
	SourceUUID *string `json:"source_uuid,omitempty"`
}

// HAEHeartRateDataPoint has Min/Avg/Max fields (capitalized in HAE JSON).
// Qty is a fallback: when the iOS app sends individual samples (not aggregated),
// only qty is set. The ingest layer promotes qty → Min/Avg/Max in that case.
type HAEHeartRateDataPoint struct {
	Date       HAETime `json:"date"`
	Min        float64 `json:"Min"`
	Avg        float64 `json:"Avg"`
	Max        float64 `json:"Max"`
	Qty        float64 `json:"qty"`
	SourceUUID *string `json:"source_uuid,omitempty"`
}

// HAEBloodPressureDataPoint has systolic/diastolic fields.
type HAEBloodPressureDataPoint struct {
	Date       HAETime `json:"date"`
	Systolic   float64 `json:"systolic"`
	Diastolic  float64 `json:"diastolic"`
	SourceUUID *string `json:"source_uuid,omitempty"`
}

// HAESleepAggregated is a nightly sleep summary (Summarize Data: ON).
type HAESleepAggregated struct {
	Date       string  `json:"date"` // date-only: "2024-02-06"
	TotalSleep float64 `json:"totalSleep"`
	Asleep     float64 `json:"asleep"`
	Core       float64 `json:"core"`
	Deep       float64 `json:"deep"`
	REM        float64 `json:"rem"`
	InBed      float64 `json:"inBed"`
	SleepStart HAETime `json:"sleepStart"`
	SleepEnd   HAETime `json:"sleepEnd"`
	InBedStart HAETime `json:"inBedStart"`
	InBedEnd   HAETime `json:"inBedEnd"`
}

// HAESleepStage is an individual sleep stage segment (Summarize Data: OFF).
type HAESleepStage struct {
	StartDate HAETime `json:"startDate"`
	EndDate   HAETime `json:"endDate"`
	Qty       float64 `json:"qty"`
	Value     string  `json:"value"`
}

// HAEWorkout is a workout from the REST API (Version 2).
type HAEWorkout struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Start    HAETime `json:"start"`
	End      HAETime `json:"end"`
	Duration float64 `json:"duration"`

	Location string `json:"location,omitempty"`
	IsIndoor *bool  `json:"isIndoor,omitempty"`

	ActiveEnergyBurned *HAEQuantity `json:"activeEnergyBurned,omitempty"`
	TotalEnergy        *HAEQuantity `json:"totalEnergy,omitempty"`
	Distance           *HAEQuantity `json:"distance,omitempty"`
	ElevationUp        *HAEQuantity `json:"elevationUp,omitempty"`
	ElevationDown      *HAEQuantity `json:"elevationDown,omitempty"`

	HeartRate *HAEHeartRateSummary `json:"heartRate,omitempty"`
	AvgHR     *HAEQuantity         `json:"avgHeartRate,omitempty"`
	MaxHR     *HAEQuantity         `json:"maxHeartRate,omitempty"`

	HeartRateData     []HAEWorkoutHRPoint `json:"heartRateData,omitempty"`
	HeartRateRecovery []HAEWorkoutHRPoint `json:"heartRateRecovery,omitempty"`
	Route             []HAERoutePoint     `json:"route,omitempty"`

	// Store original JSON for fields we don't explicitly model
	RawJSON json.RawMessage `json:"-"`
}

// HAEQuantity is the {"qty": N, "units": "..."} structure.
type HAEQuantity struct {
	Qty   float64 `json:"qty"`
	Units string  `json:"units"`
}

// HAEHeartRateSummary is the nested heartRate summary in workouts.
type HAEHeartRateSummary struct {
	Min HAEQuantity `json:"min"`
	Avg HAEQuantity `json:"avg"`
	Max HAEQuantity `json:"max"`
}

// HAEWorkoutHRPoint is a heart rate data point during a workout.
type HAEWorkoutHRPoint struct {
	Date   HAETime `json:"date"`
	Min    float64 `json:"Min"`
	Avg    float64 `json:"Avg"`
	Max    float64 `json:"Max"`
	Units  string  `json:"units"`
	Source string  `json:"source"`
}

// HAERoutePoint is a GPS point from a workout route.
type HAERoutePoint struct {
	Latitude           float64 `json:"latitude"`
	Longitude          float64 `json:"longitude"`
	Altitude           float64 `json:"altitude"`
	Course             float64 `json:"course"`
	CourseAccuracy     float64 `json:"courseAccuracy"`
	HorizontalAccuracy float64 `json:"horizontalAccuracy"`
	VerticalAccuracy   float64 `json:"verticalAccuracy"`
	Timestamp          HAETime `json:"timestamp"`
	Speed              float64 `json:"speed"`
	SpeedAccuracy      float64 `json:"speedAccuracy"`
}

// HAEECGRecording is an ECG recording from HealthBeat.
type HAEECGRecording struct {
	ID                  string    `json:"id"`
	Classification      string    `json:"classification"`
	AverageHeartRate    *float64  `json:"average_heart_rate,omitempty"`
	SamplingFrequency   *float64  `json:"sampling_frequency,omitempty"`
	VoltageMeasurements []float64 `json:"voltage_measurements,omitempty"`
	StartDate           HAETime   `json:"start_date"`
	Source              string    `json:"source,omitempty"`
}

// HAEAudiogram is an audiogram from HealthBeat.
type HAEAudiogram struct {
	ID                string               `json:"id"`
	SensitivityPoints []AudiogramSensPoint `json:"sensitivity_points"`
	StartDate         HAETime              `json:"start_date"`
	Source            string               `json:"source,omitempty"`
}

// AudiogramSensPoint is a single frequency/sensitivity measurement.
type AudiogramSensPoint struct {
	Frequency float64  `json:"hz"`
	LeftEar   *float64 `json:"left_db,omitempty"`
	RightEar  *float64 `json:"right_db,omitempty"`
}

// HAEActivitySummary is an Apple Watch activity rings summary.
type HAEActivitySummary struct {
	Date             string   `json:"date"`
	ActiveEnergy     *float64 `json:"active_energy,omitempty"`
	ActiveEnergyGoal *float64 `json:"active_energy_goal,omitempty"`
	ExerciseTime     *float64 `json:"exercise_time,omitempty"`
	ExerciseTimeGoal *float64 `json:"exercise_time_goal,omitempty"`
	StandHours       *float64 `json:"stand_hours,omitempty"`
	StandHoursGoal   *float64 `json:"stand_hours_goal,omitempty"`
}

// HAEMedication is a medication dose event.
type HAEMedication struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Dosage    *string  `json:"dosage,omitempty"`
	LogStatus *string  `json:"log_status,omitempty"`
	StartDate HAETime  `json:"start_date"`
	EndDate   *HAETime `json:"end_date,omitempty"`
	Source    string   `json:"source,omitempty"`
}

// HAEVisionPrescription is a glasses/contacts prescription.
type HAEVisionPrescription struct {
	ID               string                 `json:"id"`
	DateIssued       HAETime                `json:"date_issued"`
	ExpirationDate   *HAETime               `json:"expiration_date,omitempty"`
	PrescriptionType *string                `json:"prescription_type,omitempty"`
	RightEye         map[string]interface{} `json:"right_eye,omitempty"`
	LeftEye          map[string]interface{} `json:"left_eye,omitempty"`
	Source           string                 `json:"source,omitempty"`
}

// HAEStateOfMind is an iOS 18+ mood/emotion record.
type HAEStateOfMind struct {
	ID           string  `json:"id"`
	Kind         int     `json:"kind"`
	Valence      float64 `json:"valence"`
	Labels       []int   `json:"labels,omitempty"`
	Associations []int   `json:"associations,omitempty"`
	StartDate    HAETime `json:"start_date"`
	Source       string  `json:"source,omitempty"`
}

// HAECategorySample is an HKCategorySample record.
type HAECategorySample struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Value      int     `json:"value"`
	ValueLabel *string `json:"value_label,omitempty"`
	StartDate  HAETime `json:"start_date"`
	EndDate    HAETime `json:"end_date"`
	Source     string  `json:"source,omitempty"`
}
