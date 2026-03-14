package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// HealthTime handles the health data date format: "2006-01-02 15:04:05 -0700"
// Also handles date-only format "2006-01-02" used in aggregated sleep data.
type HealthTime struct {
	time.Time
}

const (
	HealthTimeLayout     = "2006-01-02 15:04:05 -0700"
	HealthDateOnlyLayout = "2006-01-02"
)

func (t *HealthTime) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	return t.Parse(s)
}

func (t HealthTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Format(HealthTimeLayout))
}

// Parse parses a health time string, trying full datetime first, then date-only.
func (t *HealthTime) Parse(s string) error {
	parsed, err := time.Parse(HealthTimeLayout, s)
	if err == nil {
		t.Time = parsed
		return nil
	}
	parsed, err2 := time.Parse(HealthDateOnlyLayout, s)
	if err2 == nil {
		t.Time = parsed
		return nil
	}
	return fmt.Errorf("cannot parse health time %q: %w", s, err)
}

// ParseHealthTime parses a health time string into a time.Time.
func ParseHealthTime(s string) (time.Time, error) {
	var t HealthTime
	if err := t.Parse(s); err != nil {
		return time.Time{}, err
	}
	return t.Time, nil
}

// HealthPayload is the top-level REST API JSON structure.
type HealthPayload struct {
	Data HealthData `json:"data"`
}

// HealthData contains the arrays of health data.
type HealthData struct {
	Metrics             []HealthMetric             `json:"metrics"`
	Workouts            []HealthWorkout            `json:"workouts"`
	ECGRecordings       []ECGRecording       `json:"ecg_recordings,omitempty"`
	Audiograms          []Audiogram          `json:"audiograms,omitempty"`
	ActivitySummaries   []ActivitySummary    `json:"activity_summaries,omitempty"`
	Medications         []Medication         `json:"medications,omitempty"`
	VisionPrescriptions []VisionPrescription `json:"vision_prescriptions,omitempty"`
	StateOfMind         []StateOfMind        `json:"state_of_mind,omitempty"`
	CategorySamples     []CategorySample     `json:"category_samples,omitempty"`
}

// HealthMetric is a single metric entry with name, units, and data points.
type HealthMetric struct {
	Name  string            `json:"name"`
	Units string            `json:"units"`
	Data  []json.RawMessage `json:"data"`
}

// HealthMetricDataPoint is a standard metric data point with qty.
type HealthMetricDataPoint struct {
	Date       HealthTime `json:"date"`
	Qty        float64 `json:"qty"`
	SourceUUID *string `json:"source_uuid,omitempty"`
}

// HeartRateDataPoint has Min/Avg/Max fields (capitalized in JSON).
// Qty is a fallback: when the iOS app sends individual samples (not aggregated),
// only qty is set. The ingest layer promotes qty → Min/Avg/Max in that case.
type HeartRateDataPoint struct {
	Date       HealthTime `json:"date"`
	Min        float64 `json:"Min"`
	Avg        float64 `json:"Avg"`
	Max        float64 `json:"Max"`
	Qty        float64 `json:"qty"`
	SourceUUID *string `json:"source_uuid,omitempty"`
}

// BloodPressureDataPoint has systolic/diastolic fields.
type BloodPressureDataPoint struct {
	Date       HealthTime `json:"date"`
	Systolic   float64 `json:"systolic"`
	Diastolic  float64 `json:"diastolic"`
	SourceUUID *string `json:"source_uuid,omitempty"`
}

// SleepAggregated is a nightly sleep summary (Summarize Data: ON).
type SleepAggregated struct {
	Date       string  `json:"date"` // date-only: "2024-02-06"
	TotalSleep float64 `json:"totalSleep"`
	Asleep     float64 `json:"asleep"`
	Core       float64 `json:"core"`
	Deep       float64 `json:"deep"`
	REM        float64 `json:"rem"`
	InBed      float64 `json:"inBed"`
	SleepStart HealthTime `json:"sleepStart"`
	SleepEnd   HealthTime `json:"sleepEnd"`
	InBedStart HealthTime `json:"inBedStart"`
	InBedEnd   HealthTime `json:"inBedEnd"`
}

// SleepStage is an individual sleep stage segment (Summarize Data: OFF).
type SleepStage struct {
	StartDate HealthTime `json:"startDate"`
	EndDate   HealthTime `json:"endDate"`
	Qty       float64 `json:"qty"`
	Value     string  `json:"value"`
}

// HealthWorkout is a workout from the REST API (Version 2).
type HealthWorkout struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Start    HealthTime `json:"start"`
	End      HealthTime `json:"end"`
	Duration float64 `json:"duration"`

	Location string `json:"location,omitempty"`
	IsIndoor *bool  `json:"isIndoor,omitempty"`

	ActiveEnergyBurned *Quantity `json:"activeEnergyBurned,omitempty"`
	TotalEnergy        *Quantity `json:"totalEnergy,omitempty"`
	Distance           *Quantity `json:"distance,omitempty"`
	ElevationUp        *Quantity `json:"elevationUp,omitempty"`
	ElevationDown      *Quantity `json:"elevationDown,omitempty"`

	HeartRate *HeartRateSummary `json:"heartRate,omitempty"`
	AvgHR     *Quantity         `json:"avgHeartRate,omitempty"`
	MaxHR     *Quantity         `json:"maxHeartRate,omitempty"`

	HeartRateData     []WorkoutHRPoint `json:"heartRateData,omitempty"`
	HeartRateRecovery []WorkoutHRPoint `json:"heartRateRecovery,omitempty"`
	Route             []RoutePoint     `json:"route,omitempty"`

	// Store original JSON for fields we don't explicitly model
	RawJSON json.RawMessage `json:"-"`
}

// Quantity is the {"qty": N, "units": "..."} structure.
type Quantity struct {
	Qty   float64 `json:"qty"`
	Units string  `json:"units"`
}

// HeartRateSummary is the nested heartRate summary in workouts.
type HeartRateSummary struct {
	Min Quantity `json:"min"`
	Avg Quantity `json:"avg"`
	Max Quantity `json:"max"`
}

// WorkoutHRPoint is a heart rate data point during a workout.
type WorkoutHRPoint struct {
	Date   HealthTime `json:"date"`
	Min    float64 `json:"Min"`
	Avg    float64 `json:"Avg"`
	Max    float64 `json:"Max"`
	Units  string  `json:"units"`
	Source string  `json:"source"`
}

// RoutePoint is a GPS point from a workout route.
type RoutePoint struct {
	Latitude           float64 `json:"latitude"`
	Longitude          float64 `json:"longitude"`
	Altitude           float64 `json:"altitude"`
	Course             float64 `json:"course"`
	CourseAccuracy     float64 `json:"courseAccuracy"`
	HorizontalAccuracy float64 `json:"horizontalAccuracy"`
	VerticalAccuracy   float64 `json:"verticalAccuracy"`
	Timestamp          HealthTime `json:"timestamp"`
	Speed              float64 `json:"speed"`
	SpeedAccuracy      float64 `json:"speedAccuracy"`
}

// ECGRecording is an ECG recording from HealthBeat.
type ECGRecording struct {
	ID                  string    `json:"id"`
	Classification      string    `json:"classification"`
	AverageHeartRate    *float64  `json:"average_heart_rate,omitempty"`
	SamplingFrequency   *float64  `json:"sampling_frequency,omitempty"`
	VoltageMeasurements []float64 `json:"voltage_measurements,omitempty"`
	StartDate           HealthTime   `json:"start_date"`
	Source              string    `json:"source,omitempty"`
}

// Audiogram is an audiogram from HealthBeat.
type Audiogram struct {
	ID                string               `json:"id"`
	SensitivityPoints []AudiogramSensPoint `json:"sensitivity_points"`
	StartDate         HealthTime              `json:"start_date"`
	Source            string               `json:"source,omitempty"`
}

// AudiogramSensPoint is a single frequency/sensitivity measurement.
type AudiogramSensPoint struct {
	Frequency float64  `json:"hz"`
	LeftEar   *float64 `json:"left_db,omitempty"`
	RightEar  *float64 `json:"right_db,omitempty"`
}

// ActivitySummary is an Apple Watch activity rings summary.
type ActivitySummary struct {
	Date             string   `json:"date"`
	ActiveEnergy     *float64 `json:"active_energy,omitempty"`
	ActiveEnergyGoal *float64 `json:"active_energy_goal,omitempty"`
	ExerciseTime     *float64 `json:"exercise_time,omitempty"`
	ExerciseTimeGoal *float64 `json:"exercise_time_goal,omitempty"`
	StandHours       *float64 `json:"stand_hours,omitempty"`
	StandHoursGoal   *float64 `json:"stand_hours_goal,omitempty"`
}

// Medication is a medication dose event.
type Medication struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Dosage    *string  `json:"dosage,omitempty"`
	LogStatus *string  `json:"log_status,omitempty"`
	StartDate HealthTime  `json:"start_date"`
	EndDate   *HealthTime `json:"end_date,omitempty"`
	Source    string   `json:"source,omitempty"`
}

// VisionPrescription is a glasses/contacts prescription.
type VisionPrescription struct {
	ID               string                 `json:"id"`
	DateIssued       HealthTime                `json:"date_issued"`
	ExpirationDate   *HealthTime               `json:"expiration_date,omitempty"`
	PrescriptionType *string                `json:"prescription_type,omitempty"`
	RightEye         map[string]interface{} `json:"right_eye,omitempty"`
	LeftEye          map[string]interface{} `json:"left_eye,omitempty"`
	Source           string                 `json:"source,omitempty"`
}

// StateOfMind is an iOS 18+ mood/emotion record.
type StateOfMind struct {
	ID           string  `json:"id"`
	Kind         int     `json:"kind"`
	Valence      float64 `json:"valence"`
	Labels       []int   `json:"labels,omitempty"`
	Associations []int   `json:"associations,omitempty"`
	StartDate    HealthTime `json:"start_date"`
	Source       string  `json:"source,omitempty"`
}

// CategorySample is an HKCategorySample record.
type CategorySample struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Value      int     `json:"value"`
	ValueLabel *string `json:"value_label,omitempty"`
	StartDate  HealthTime `json:"start_date"`
	EndDate    HealthTime `json:"end_date"`
	Source     string  `json:"source,omitempty"`
}
