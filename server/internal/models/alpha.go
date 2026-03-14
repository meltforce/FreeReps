package models

import "time"

// AlphaSession represents a parsed Alpha Progression workout session.
type AlphaSession struct {
	Name      string
	Date      time.Time
	Duration  string
	Exercises []AlphaExercise
}

// AlphaExercise represents a single exercise within a session.
type AlphaExercise struct {
	Number     int
	Name       string
	Equipment  string
	TargetReps int
	Sets       []AlphaSet
}

// AlphaSet represents a single set (working or warmup).
type AlphaSet struct {
	Number           int
	WeightKg         float64
	IsBodyweightPlus bool
	Reps             int
	RIR              float64
	IsWarmup         bool
}
