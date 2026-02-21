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
	Time                time.Time
	WorkoutID           uuid.UUID
	UserID              int
	Latitude            float64
	Longitude           float64
	Altitude            *float64
	Speed               *float64
	Course              *float64
	HorizontalAccuracy  *float64
	VerticalAccuracy    *float64
}

// WorkoutSetRow is a row for the workout_sets table.
type WorkoutSetRow struct {
	UserID           int
	SessionName      string
	SessionDate      time.Time
	SessionDuration  string
	ExerciseNumber   int
	ExerciseName     string
	Equipment        string
	TargetReps       int
	IsWarmup         bool
	SetNumber        int
	WeightKg         float64
	IsBodyweightPlus bool
	Reps             int
	RIR              float64
}
