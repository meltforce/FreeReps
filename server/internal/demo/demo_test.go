package demo

import (
	"math/rand"
	"testing"
	"time"
)

// TestGenerateHealthMetrics verifies that the health metric generator produces
// the expected volume of data points across all metric types for 90 days.
func TestGenerateHealthMetrics(t *testing.T) {
	rng := rand.New(rand.NewSource(randSeed))
	end := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	start := end.AddDate(0, 0, -daysBack)

	rows := generateHealthMetrics(rng, start, end)
	if len(rows) == 0 {
		t.Fatal("expected health metrics, got none")
	}

	// Count by metric type
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.MetricName]++
	}

	// Heart rate: ~180 samples/day (15h * 12 per hour) * 90 days = ~16,200
	if c := counts["heart_rate"]; c < 15000 || c > 17000 {
		t.Errorf("heart_rate count = %d, want ~16200", c)
	}

	// Resting heart rate: 1 per day = 90
	if c := counts["resting_heart_rate"]; c != daysBack {
		t.Errorf("resting_heart_rate count = %d, want %d", c, daysBack)
	}

	// Step count: 15 hours/day * 90 = 1350
	if c := counts["step_count"]; c < 1200 || c > 1500 {
		t.Errorf("step_count count = %d, want ~1350", c)
	}

	// Active energy: same as step count
	if c := counts["active_energy"]; c < 1200 || c > 1500 {
		t.Errorf("active_energy count = %d, want ~1350", c)
	}

	// Weight: ~13 Sundays in 90 days
	if c := counts["weight_body_mass"]; c < 10 || c > 15 {
		t.Errorf("weight_body_mass count = %d, want ~13", c)
	}

	// Daily metrics: blood oxygen, respiratory rate, HRV
	for _, name := range []string{"blood_oxygen_saturation", "respiratory_rate", "heart_rate_variability"} {
		if c := counts[name]; c != daysBack {
			t.Errorf("%s count = %d, want %d", name, c, daysBack)
		}
	}

	// VO2 Max: ~13 Mondays in 90 days
	if c := counts["vo2_max"]; c < 10 || c > 15 {
		t.Errorf("vo2_max count = %d, want ~13", c)
	}
}

// TestGenerateSleep verifies that sleep generation produces one session per
// night with valid stage breakdowns.
func TestGenerateSleep(t *testing.T) {
	rng := rand.New(rand.NewSource(randSeed))
	end := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	start := end.AddDate(0, 0, -daysBack)

	sessions, stages := generateSleep(rng, start, end)

	if len(sessions) != daysBack {
		t.Errorf("sessions = %d, want %d", len(sessions), daysBack)
	}

	if len(stages) == 0 {
		t.Fatal("expected sleep stages, got none")
	}

	// Check first session has valid values
	s := sessions[0]
	if s.TotalSleep < 6.0 || s.TotalSleep > 9.0 {
		t.Errorf("total sleep = %.1f, want 6-9h", s.TotalSleep)
	}
	if s.Deep <= 0 || s.REM <= 0 || s.Core <= 0 {
		t.Errorf("stage breakdown should be positive: deep=%.2f rem=%.2f core=%.2f", s.Deep, s.REM, s.Core)
	}
	if s.SleepStart.After(s.SleepEnd) {
		t.Errorf("sleep start %v after end %v", s.SleepStart, s.SleepEnd)
	}

	// Verify stages reference valid stage names
	validStages := map[string]bool{"Core": true, "Deep": true, "REM": true, "Awake": true}
	for _, st := range stages {
		if !validStages[st.Stage] {
			t.Errorf("unexpected stage name: %q", st.Stage)
			break
		}
	}
}

// TestGenerateWorkouts verifies that workout generation produces a reasonable
// number of workouts with valid heart rate data.
func TestGenerateWorkouts(t *testing.T) {
	rng := rand.New(rand.NewSource(randSeed))
	end := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	start := end.AddDate(0, 0, -daysBack)

	workouts, hrRows := generateWorkouts(rng, start, end)

	// ~50% of days → ~45 workouts
	if len(workouts) < 30 || len(workouts) > 60 {
		t.Errorf("workouts = %d, want 30-60", len(workouts))
	}

	if len(hrRows) == 0 {
		t.Fatal("expected workout HR rows, got none")
	}

	// Check first workout
	w := workouts[0]
	if w.DurationSec < 1200 || w.DurationSec > 5400 {
		t.Errorf("duration = %.0f, want 1200-5400", w.DurationSec)
	}
	if w.AvgHeartRate == nil || *w.AvgHeartRate < 60 || *w.AvgHeartRate > 200 {
		t.Errorf("avg HR = %v, want 60-200", w.AvgHeartRate)
	}

	// Verify all workout types are represented
	types := map[string]bool{}
	for _, w := range workouts {
		types[w.Name] = true
	}
	for _, tmpl := range workoutTemplates {
		if !types[tmpl.name] {
			t.Errorf("missing workout type: %s", tmpl.name)
		}
	}
}

// TestGenerateActivitySummaries verifies that activity ring data is generated
// for every day in the range.
func TestGenerateActivitySummaries(t *testing.T) {
	rng := rand.New(rand.NewSource(randSeed))
	end := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	start := end.AddDate(0, 0, -daysBack)

	rows := generateActivitySummaries(rng, start, end)

	if len(rows) != daysBack {
		t.Errorf("activity summaries = %d, want %d", len(rows), daysBack)
	}

	// Check first row has valid values
	r := rows[0]
	if r.ActiveEnergy == nil || *r.ActiveEnergy < 100 || *r.ActiveEnergy > 1000 {
		t.Errorf("active energy = %v, want 100-1000", r.ActiveEnergy)
	}
	if r.ExerciseTime == nil || *r.ExerciseTime < 10 || *r.ExerciseTime > 60 {
		t.Errorf("exercise time = %v, want 10-60", r.ExerciseTime)
	}
	if r.StandHours == nil || *r.StandHours < 5 || *r.StandHours > 16 {
		t.Errorf("stand hours = %v, want 5-16", r.StandHours)
	}
}
