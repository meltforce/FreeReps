package models

import (
	"time"

	"github.com/google/uuid"
)

// HealthMetricRow is a row ready for insertion into the health_metrics table.
type HealthMetricRow struct {
	Time       time.Time
	UserID     int
	MetricName string
	Source     string
	Units      string
	Qty        *float64
	MinVal     *float64
	AvgVal     *float64
	MaxVal     *float64
	Systolic   *float64
	Diastolic  *float64
	SourceUUID *uuid.UUID
}

// SleepSessionRow is a row ready for insertion into the sleep_sessions table.
type SleepSessionRow struct {
	UserID     int
	Date       time.Time
	TotalSleep float64
	Asleep     float64
	Core       float64
	Deep       float64
	REM        float64
	InBed      float64
	SleepStart time.Time
	SleepEnd   time.Time
	InBedStart time.Time
	InBedEnd   time.Time
}

// SleepStageRow is a row ready for insertion into the sleep_stages table.
type SleepStageRow struct {
	StartTime  time.Time
	EndTime    time.Time
	UserID     int
	Stage      string
	DurationHr float64
	Source     string
}

// WorkoutRow is a row ready for insertion into the workouts table.
type WorkoutRow struct {
	ID                 uuid.UUID
	UserID             int
	Name               string
	Source             string
	StartTime          time.Time
	EndTime            time.Time
	DurationSec        float64
	Location           string
	IsIndoor           *bool
	ActiveEnergyBurned *float64
	ActiveEnergyUnits  string
	TotalEnergy        *float64
	TotalEnergyUnits   string
	Distance           *float64
	DistanceUnits      string
	AvgHeartRate       *float64
	MaxHeartRate       *float64
	MinHeartRate       *float64
	ElevationUp        *float64
	ElevationDown      *float64
	RawJSON            []byte `json:"-"`
	AlphaSessionName   string `json:"alpha_session_name,omitempty"`

	// Sources names every source that reported this workout, which is more
	// than one whenever a provider writes its session into HealthKit as well
	// and a second client forwards it. The list survives the deduplication in
	// QueryWorkouts, where only the highest-priority row is returned, so the
	// display can name the provider that recorded a session rather than the
	// hub it arrived through. Empty on the single-row path.
	Sources []string `json:"Sources,omitempty"`

	// RecordedBy is the name to show for this workout, resolved from Source and
	// Sources by RecordedBy(). Carried on the row so that the workout list and
	// the MCP tool report the same provider; see DECISIONS.md, 2026-09-21.
	//
	// Absent on the single-row path: GetWorkout selects no source column, so
	// resolving there would label every workout "Apple Health".
	RecordedBy string `json:"RecordedBy,omitempty"`
}

// WorkoutHRRow is a row for the workout_heart_rate table.
type WorkoutHRRow struct {
	Time      time.Time
	WorkoutID uuid.UUID
	UserID    int
	MinBPM    *float64
	AvgBPM    *float64
	MaxBPM    *float64
	Source    string
}

// WorkoutRouteRow is a row for the workout_routes table.
type WorkoutRouteRow struct {
	Time               time.Time
	WorkoutID          uuid.UUID
	UserID             int
	Latitude           float64
	Longitude          float64
	Altitude           *float64
	Speed              *float64
	Course             *float64
	HorizontalAccuracy *float64
	VerticalAccuracy   *float64
}

// WorkoutSetRow is a row for the workout_sets table.
//
// Two sources write here. Alpha Progression fills Equipment, TargetReps and
// IsBodyweightPlus and leaves the Hevy-specific fields empty; Hevy fills
// ExternalID, RoutineID, ExerciseTemplateID, SetType and SupersetID and leaves
// the Alpha-specific ones empty. Source distinguishes them and is part of the
// natural key.
//
// Effort is recorded on whichever scale the source uses: Alpha writes RIR (with
// -1 for an unrated set), Hevy writes RPE. EffortRIR is computed by the database
// from whichever is present and is the field every query should read.
type WorkoutSetRow struct {
	UserID             int
	Source             string
	ExternalID         string
	RoutineID          string
	SessionName        string
	SessionDate        time.Time
	SessionEnd         *time.Time
	SessionDuration    string
	ExerciseNumber     int
	ExerciseName       string
	ExerciseTemplateID string
	ExerciseNotes      string
	// PrimaryMuscleGroup is resolved from the exercise catalog on read and is
	// never written. Empty when the exercise reaches no catalog entry.
	PrimaryMuscleGroup string
	Equipment          string
	TargetReps         int
	IsWarmup           bool
	SetType            string
	SetNumber          int
	SupersetID         *int
	WeightKg           float64
	IsBodyweightPlus   bool
	Reps               int
	RIR                *float64
	RPE                *float64
	EffortRIR          *float64
	DistanceM          *float64
	DurationSec        *float64
	CustomMetric       *float64
}

// ECGRecordingRow is a row for the ecg_recordings table.
type ECGRecordingRow struct {
	ID                  uuid.UUID
	UserID              int
	Classification      string
	AverageHeartRate    *float64
	SamplingFrequency   *float64
	VoltageMeasurements []byte // raw JSON array of floats
	StartDate           time.Time
	Source              string
}

// AudiogramRow is a row for the audiograms table.
type AudiogramRow struct {
	ID                uuid.UUID
	UserID            int
	SensitivityPoints []byte // raw JSON array of {hz, left_db, right_db}
	StartDate         time.Time
	Source            string
}

// ActivitySummaryRow is a row for the activity_summaries table.
type ActivitySummaryRow struct {
	UserID           int
	Date             time.Time
	ActiveEnergy     *float64
	ActiveEnergyGoal *float64
	ExerciseTime     *float64
	ExerciseTimeGoal *float64
	StandHours       *float64
	StandHoursGoal   *float64
}

// MedicationRow is a row for the medications table.
type MedicationRow struct {
	ID        uuid.UUID
	UserID    int
	Name      string
	Dosage    *string
	LogStatus *string
	StartDate time.Time
	EndDate   *time.Time
	Source    string
}

// VisionPrescriptionRow is a row for the vision_prescriptions table.
type VisionPrescriptionRow struct {
	ID               uuid.UUID
	UserID           int
	DateIssued       time.Time
	ExpirationDate   *time.Time
	PrescriptionType *string
	RightEye         []byte // raw JSON
	LeftEye          []byte // raw JSON
	Source           string
}

// StateOfMindRow is a row for the state_of_mind table.
type StateOfMindRow struct {
	ID           uuid.UUID
	UserID       int
	Kind         int
	Valence      float64
	Labels       []int
	Associations []int
	StartDate    time.Time
	Source       string
}

// CategorySampleRow is a row for the category_samples table.
type CategorySampleRow struct {
	ID         uuid.UUID
	UserID     int
	Type       string
	Value      int
	ValueLabel *string
	StartDate  time.Time
	EndDate    time.Time
	Source     string
}
