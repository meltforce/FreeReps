package oura

import (
	"testing"
	"time"
)

// TestParseSleepPhases verifies the core sleep phase parser that converts Oura's
// compact 5-minute phase string into merged sleep stage rows. This is the most
// complex mapping and the most likely to have edge cases.
func TestParseSleepPhases(t *testing.T) {
	bedtime := time.Date(2024, 1, 15, 22, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		phases     string
		wantStages int
		wantFirst  string
		wantLast   string
	}{
		{
			name:       "mixed phases merge consecutive segments",
			phases:     "1112233344",
			wantStages: 4,
			wantFirst:  "Deep",
			wantLast:   "Awake",
		},
		{
			name:       "empty string produces no stages",
			phases:     "",
			wantStages: 0,
		},
		{
			name:       "single character produces one stage",
			phases:     "1",
			wantStages: 1,
			wantFirst:  "Deep",
			wantLast:   "Deep",
		},
		{
			name:       "all same type merges into one stage",
			phases:     "33333",
			wantStages: 1,
			wantFirst:  "REM",
			wantLast:   "REM",
		},
		{
			name:       "light maps to Core (FreeReps convention)",
			phases:     "2",
			wantStages: 1,
			wantFirst:  "Core",
			wantLast:   "Core",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stages := parseSleepPhases(tt.phases, bedtime, 1)
			if len(stages) != tt.wantStages {
				t.Fatalf("got %d stages, want %d", len(stages), tt.wantStages)
			}
			if tt.wantStages == 0 {
				return
			}
			if stages[0].Stage != tt.wantFirst {
				t.Errorf("first stage = %q, want %q", stages[0].Stage, tt.wantFirst)
			}
			if stages[len(stages)-1].Stage != tt.wantLast {
				t.Errorf("last stage = %q, want %q", stages[len(stages)-1].Stage, tt.wantLast)
			}
			// Verify all stages have Source = "Oura"
			for _, s := range stages {
				if s.Source != "Oura" {
					t.Errorf("stage source = %q, want Oura", s.Source)
				}
			}
		})
	}

	// Detailed check: "1112233344" = 1,1,1,2,2,3,3,3,4,4
	// Deep: 22:00-22:15 (15min), Core: 22:15-22:25 (10min), REM: 22:25-22:40 (15min), Awake: 22:40-22:50 (10min)
	t.Run("detailed timing", func(t *testing.T) {
		stages := parseSleepPhases("1112233344", bedtime, 1)
		if len(stages) != 4 {
			t.Fatalf("got %d stages, want 4", len(stages))
		}
		// Deep: 3 segments = 15 minutes = 0.25 hr
		if got := stages[0].DurationHr; got != 0.25 {
			t.Errorf("Deep duration = %f hr, want 0.25", got)
		}
		// Core: 2 segments = 10 minutes
		if got := stages[1].DurationHr; !approxEqual(got, 10.0/60) {
			t.Errorf("Core duration = %f hr, want %f", got, 10.0/60)
		}
		// REM: 3 segments = 15 minutes
		if got := stages[2].DurationHr; got != 0.25 {
			t.Errorf("REM duration = %f hr, want 0.25", got)
		}
		// Awake starts at segment 8 = minute 40
		wantAwakeStart := bedtime.Add(40 * time.Minute)
		if !stages[3].StartTime.Equal(wantAwakeStart) {
			t.Errorf("Awake start = %v, want %v", stages[3].StartTime, wantAwakeStart)
		}
	})
}

// TestMapDailyReadiness verifies that readiness scores and temperature deviation
// are correctly mapped to separate health metric rows.
func TestMapDailyReadiness(t *testing.T) {
	score := 85
	tempDev := 0.12
	items := []DailyReadinessItem{
		{ID: "r1", Day: "2024-01-15", Score: &score, TemperatureDeviation: &tempDev},
		{ID: "r2", Day: "2024-01-16"}, // nil score and temp — should produce no rows
	}

	rows := MapDailyReadiness(items, 1)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (score + temp_dev from first item)", len(rows))
	}
	if rows[0].MetricName != "oura_readiness_score" {
		t.Errorf("row[0].MetricName = %q, want oura_readiness_score", rows[0].MetricName)
	}
	if *rows[0].Qty != 85 {
		t.Errorf("row[0].Qty = %f, want 85", *rows[0].Qty)
	}
	if rows[1].MetricName != "oura_temperature_deviation" {
		t.Errorf("row[1].MetricName = %q, want oura_temperature_deviation", rows[1].MetricName)
	}
}

// TestMapDailyActivity verifies that steps and calories are mapped to the
// correct FreeReps cumulative metric names.
func TestMapDailyActivity(t *testing.T) {
	score := 90
	items := []DailyActivityItem{
		{Day: "2024-01-15", Score: &score, Steps: 8500, ActiveCalories: 350},
	}

	rows := MapDailyActivity(items, 1)
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3 (score + steps + calories)", len(rows))
	}

	names := map[string]float64{}
	for _, r := range rows {
		names[r.MetricName] = *r.Qty
	}
	if v, ok := names["step_count"]; !ok || v != 8500 {
		t.Errorf("step_count = %v, want 8500", v)
	}
	if v, ok := names["active_energy"]; !ok || v != 350 {
		t.Errorf("active_energy = %v, want 350", v)
	}
}

// TestMapSleepSessionsOnlyLongSleep verifies that only "long_sleep" sessions
// produce sleep session rows — naps ("rest") and "deleted" entries are excluded,
// preventing short naps from overwriting main sleep via DO UPDATE.
func TestMapSleepSessionsOnlyLongSleep(t *testing.T) {
	mainDur := 25200 // 7 hours
	napDur := 1800   // 30 minutes
	items := []SleepItem{
		{Day: "2024-01-15", Type: "long_sleep", BedtimeStart: "2024-01-14T23:00:00", BedtimeEnd: "2024-01-15T07:00:00", TimeInBed: 28800, TotalSleepDuration: &mainDur},
		{Day: "2024-01-15", Type: "rest", BedtimeStart: "2024-01-15T13:00:00", BedtimeEnd: "2024-01-15T13:30:00", TimeInBed: 1800, TotalSleepDuration: &napDur, SleepPhase5Min: strPtr("222222")},
		{Day: "2024-01-15", Type: "deleted", BedtimeStart: "2024-01-14T23:00:00", BedtimeEnd: "2024-01-15T07:00:00", TimeInBed: 28800},
	}

	sessions, stages := MapSleepSessions(items, 1)
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1 (only long_sleep)", len(sessions))
	}
	if sessions[0].TotalSleep != 7.0 {
		t.Errorf("TotalSleep = %f, want 7.0", sessions[0].TotalSleep)
	}
	if sessions[0].InBed != 8.0 {
		t.Errorf("InBed = %f, want 8.0", sessions[0].InBed)
	}
	// Stages should still include the nap's phases (rest is not deleted).
	if len(stages) == 0 {
		t.Error("expected sleep stages from nap, got none")
	}
}

func strPtr(s string) *string { return &s }

// TestMapDailyResilience verifies the string-to-numeric level encoding.
func TestMapDailyResilience(t *testing.T) {
	items := []DailyResilienceItem{
		{Day: "2024-01-15", Level: "limited"},
		{Day: "2024-01-16", Level: "exceptional"},
		{Day: "2024-01-17", Level: "unknown"}, // should be skipped
	}

	rows := MapDailyResilience(items, 1)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (unknown should be skipped)", len(rows))
	}
	if *rows[0].Qty != 1 {
		t.Errorf("limited = %f, want 1", *rows[0].Qty)
	}
	if *rows[1].Qty != 5 {
		t.Errorf("exceptional = %f, want 5", *rows[1].Qty)
	}
}

// TestMapDailyCardiovascularAge verifies nullable vascular_age handling.
func TestMapDailyCardiovascularAge(t *testing.T) {
	age := 35
	items := []DailyCardiovascularAgeItem{
		{Day: "2024-01-15", VascularAge: &age},
		{Day: "2024-01-16", VascularAge: nil}, // should be skipped
	}

	rows := MapDailyCardiovascularAge(items, 1)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if *rows[0].Qty != 35 {
		t.Errorf("vascular age = %f, want 35", *rows[0].Qty)
	}
}

// TestOuraWorkoutUUID verifies that the same Oura workout ID always produces
// the same UUID, ensuring idempotent re-syncs.
func TestOuraWorkoutUUID(t *testing.T) {
	id1 := ouraWorkoutUUID("workout-abc-123")
	id2 := ouraWorkoutUUID("workout-abc-123")
	id3 := ouraWorkoutUUID("workout-xyz-456")

	if id1 != id2 {
		t.Errorf("same input produced different UUIDs: %s vs %s", id1, id2)
	}
	if id1 == id3 {
		t.Error("different inputs produced the same UUID")
	}
}

// TestAllMappersSetOuraSource verifies that every mapper sets Source to "Oura",
// which is critical for source-priority deduplication.
func TestAllMappersSetOuraSource(t *testing.T) {
	score := 80
	readiness := MapDailyReadiness([]DailyReadinessItem{{Day: "2024-01-15", Score: &score}}, 1)
	for _, r := range readiness {
		if r.Source != "Oura" {
			t.Errorf("readiness source = %q, want Oura", r.Source)
		}
	}

	hr := MapHeartRate([]HeartRateItem{{BPM: 72, Timestamp: "2024-01-15T03:00:00+00:00"}}, 1)
	for _, r := range hr {
		if r.Source != "Oura" {
			t.Errorf("heart_rate source = %q, want Oura", r.Source)
		}
	}
}

func approxEqual(a, b float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < 0.0001
}
