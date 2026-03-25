package oura

import (
	"encoding/json"
	"time"

	"github.com/claude/freereps/internal/ingest"
	"github.com/claude/freereps/internal/models"
	"github.com/google/uuid"
)

const ouraSource = "Oura"

// ouraWorkoutNamespace is the UUID namespace for deterministic Oura workout IDs.
var ouraWorkoutNamespace = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")

// resilience level string → numeric encoding.
var resilienceLevels = map[string]float64{
	"limited":     1,
	"adequate":    2,
	"solid":       3,
	"strong":      4,
	"exceptional": 5,
}

// parseDay parses an Oura "YYYY-MM-DD" day string into a time.Time at noon UTC.
func parseDay(day string) time.Time {
	t, _ := time.Parse("2006-01-02", day)
	return t.Add(12 * time.Hour) // noon UTC so bucketing works
}

// parseLocalDatetime parses an Oura ISO 8601 local datetime string.
func parseLocalDatetime(s string) time.Time {
	// Oura uses various formats: "2024-01-15T03:00:00+02:00" or "2024-01-15T03:00:00"
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func floatPtr(v float64) *float64  { return &v }
func intToFloat(v int) *float64    { return floatPtr(float64(v)) }

func metricRow(t time.Time, userID int, name, units string, qty *float64) models.HealthMetricRow {
	return models.HealthMetricRow{
		Time:       t,
		UserID:     userID,
		MetricName: name,
		Source:     ouraSource,
		Units:      units,
		Qty:        qty,
	}
}

// MapDailyReadiness converts Oura daily readiness scores and temperature deviation.
func MapDailyReadiness(items []DailyReadinessItem, userID int) []models.HealthMetricRow {
	var rows []models.HealthMetricRow
	for _, item := range items {
		t := parseDay(item.Day)
		if item.Score != nil {
			rows = append(rows, metricRow(t, userID, "oura_readiness_score", "score", intToFloat(*item.Score)))
		}
		if item.TemperatureDeviation != nil {
			rows = append(rows, metricRow(t, userID, "oura_temperature_deviation", "degC", item.TemperatureDeviation))
		}
	}
	return rows
}

// MapDailySleep converts Oura daily sleep scores.
func MapDailySleep(items []DailySleepItem, userID int) []models.HealthMetricRow {
	var rows []models.HealthMetricRow
	for _, item := range items {
		if item.Score != nil {
			rows = append(rows, metricRow(parseDay(item.Day), userID, "oura_sleep_score", "score", intToFloat(*item.Score)))
		}
	}
	return rows
}

// MapDailyActivity converts Oura daily activity scores, steps, and calories.
func MapDailyActivity(items []DailyActivityItem, userID int) []models.HealthMetricRow {
	var rows []models.HealthMetricRow
	for _, item := range items {
		t := parseDay(item.Day)
		if item.Score != nil {
			rows = append(rows, metricRow(t, userID, "oura_activity_score", "score", intToFloat(*item.Score)))
		}
		if item.Steps > 0 {
			rows = append(rows, metricRow(t, userID, "step_count", "count", intToFloat(item.Steps)))
		}
		if item.ActiveCalories > 0 {
			rows = append(rows, metricRow(t, userID, "active_energy", "kcal", intToFloat(item.ActiveCalories)))
		}
	}
	return rows
}

// MapSleepSessions converts Oura sleep data to FreeReps sleep sessions and stages.
// Filters out sessions with type "deleted".
func MapSleepSessions(items []SleepItem, userID int) ([]models.SleepSessionRow, []models.SleepStageRow) {
	var sessions []models.SleepSessionRow
	var stages []models.SleepStageRow

	for _, item := range items {
		if item.Type == "deleted" {
			continue
		}

		date, _ := time.Parse("2006-01-02", item.Day)
		bedStart := parseLocalDatetime(item.BedtimeStart)
		bedEnd := parseLocalDatetime(item.BedtimeEnd)

		session := models.SleepSessionRow{
			UserID:     userID,
			Date:       date,
			InBed:      float64(item.TimeInBed) / 3600,
			SleepStart: bedStart,
			SleepEnd:   bedEnd,
			InBedStart: bedStart,
			InBedEnd:   bedEnd,
		}
		if item.TotalSleepDuration != nil {
			session.TotalSleep = float64(*item.TotalSleepDuration) / 3600
			session.Asleep = session.TotalSleep
		}
		if item.DeepSleepDuration != nil {
			session.Deep = float64(*item.DeepSleepDuration) / 3600
		}
		if item.LightSleepDuration != nil {
			session.Core = float64(*item.LightSleepDuration) / 3600
		}
		if item.REMSleepDuration != nil {
			session.REM = float64(*item.REMSleepDuration) / 3600
		}
		sessions = append(sessions, session)

		// Parse 5-minute sleep phases into individual stage rows.
		if item.SleepPhase5Min != nil {
			stages = append(stages, parseSleepPhases(*item.SleepPhase5Min, bedStart, userID)...)
		}
	}
	return sessions, stages
}

// MapHeartRate converts Oura heart rate time-series to health metric rows.
func MapHeartRate(items []HeartRateItem, userID int) []models.HealthMetricRow {
	rows := make([]models.HealthMetricRow, 0, len(items))
	for _, item := range items {
		t := parseLocalDatetime(item.Timestamp)
		if t.IsZero() {
			continue
		}
		rows = append(rows, metricRow(t, userID, "heart_rate", "bpm", intToFloat(item.BPM)))
	}
	return rows
}

// MapDailySpO2 converts Oura SpO2 averages.
func MapDailySpO2(items []DailySpO2Item, userID int) []models.HealthMetricRow {
	var rows []models.HealthMetricRow
	for _, item := range items {
		if item.SpO2Percentage == nil {
			continue
		}
		rows = append(rows, metricRow(parseDay(item.Day), userID, "blood_oxygen_saturation", "%", floatPtr(item.SpO2Percentage.Average)))
	}
	return rows
}

// MapDailyStress converts Oura stress and recovery seconds.
func MapDailyStress(items []DailyStressItem, userID int) []models.HealthMetricRow {
	var rows []models.HealthMetricRow
	for _, item := range items {
		t := parseDay(item.Day)
		if item.StressHigh != nil {
			rows = append(rows, metricRow(t, userID, "oura_stress_high", "s", intToFloat(*item.StressHigh)))
		}
		if item.RecoveryHigh != nil {
			rows = append(rows, metricRow(t, userID, "oura_recovery_high", "s", intToFloat(*item.RecoveryHigh)))
		}
	}
	return rows
}

// MapDailyResilience converts Oura resilience levels to numeric values (1-5).
func MapDailyResilience(items []DailyResilienceItem, userID int) []models.HealthMetricRow {
	var rows []models.HealthMetricRow
	for _, item := range items {
		level, ok := resilienceLevels[item.Level]
		if !ok {
			continue
		}
		rows = append(rows, metricRow(parseDay(item.Day), userID, "oura_resilience", "level", floatPtr(level)))
	}
	return rows
}

// MapDailyCardiovascularAge converts Oura predicted vascular age.
func MapDailyCardiovascularAge(items []DailyCardiovascularAgeItem, userID int) []models.HealthMetricRow {
	var rows []models.HealthMetricRow
	for _, item := range items {
		if item.VascularAge == nil {
			continue
		}
		rows = append(rows, metricRow(parseDay(item.Day), userID, "oura_cardiovascular_age", "years", intToFloat(*item.VascularAge)))
	}
	return rows
}

// MapVO2Max converts Oura VO2 max estimates.
func MapVO2Max(items []VO2MaxItem, userID int) []models.HealthMetricRow {
	var rows []models.HealthMetricRow
	for _, item := range items {
		if item.VO2Max == nil {
			continue
		}
		rows = append(rows, metricRow(parseDay(item.Day), userID, "vo2_max", "mL/kg/min", item.VO2Max))
	}
	return rows
}

// MapWorkouts converts Oura workouts to FreeReps workout rows.
func MapWorkouts(items []WorkoutItem, userID int) []models.WorkoutRow {
	var rows []models.WorkoutRow
	for _, item := range items {
		start := parseLocalDatetime(item.StartDatetime)
		end := parseLocalDatetime(item.EndDatetime)
		if start.IsZero() || end.IsZero() {
			continue
		}

		name := ingest.NormalizeWorkoutName(item.Activity)
		if item.Label != nil && *item.Label != "" {
			name = *item.Label
		}

		raw, _ := json.Marshal(item)

		row := models.WorkoutRow{
			ID:                ouraWorkoutUUID(item.ID),
			UserID:            userID,
			Name:              name,
			Source:            ouraSource,
			StartTime:         start,
			EndTime:           end,
			DurationSec:       end.Sub(start).Seconds(),
			ActiveEnergyBurned: item.Calories,
			ActiveEnergyUnits: "kcal",
			Distance:          item.Distance,
			DistanceUnits:     "m",
			RawJSON:           raw,
		}
		rows = append(rows, row)
	}
	return rows
}

// ouraWorkoutUUID generates a deterministic UUID from an Oura workout ID.
func ouraWorkoutUUID(ouraID string) uuid.UUID {
	return uuid.NewSHA1(ouraWorkoutNamespace, []byte("oura:workout:"+ouraID))
}

// parseSleepPhases parses the Oura sleep_phase_5_min string into sleep stage rows.
// Each character represents 5 minutes: '1'=Deep, '2'=Core, '3'=REM, '4'=Awake.
// Consecutive segments of the same type are merged into a single row.
func parseSleepPhases(phases string, bedtimeStart time.Time, userID int) []models.SleepStageRow {
	if len(phases) == 0 {
		return nil
	}

	stageNames := map[byte]string{
		'1': "Deep",
		'2': "Core",
		'3': "REM",
		'4': "Awake",
	}

	var stages []models.SleepStageRow
	segStart := 0
	for i := 1; i <= len(phases); i++ {
		if i < len(phases) && phases[i] == phases[segStart] {
			continue
		}
		name, ok := stageNames[phases[segStart]]
		if !ok {
			segStart = i
			continue
		}

		startTime := bedtimeStart.Add(time.Duration(segStart) * 5 * time.Minute)
		endTime := bedtimeStart.Add(time.Duration(i) * 5 * time.Minute)
		durationHr := endTime.Sub(startTime).Hours()

		stages = append(stages, models.SleepStageRow{
			StartTime:  startTime,
			EndTime:    endTime,
			UserID:     userID,
			Stage:      name,
			DurationHr: durationHr,
			Source:     ouraSource,
		})
		segStart = i
	}
	return stages
}
