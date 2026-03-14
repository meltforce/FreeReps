package storage

import (
	"context"
	"fmt"
	"time"
)

// WorkoutTypePeriodSummary holds aggregated workout stats for one type within a period.
type WorkoutTypePeriodSummary struct {
	Type         string   `json:"type"`
	Count        int      `json:"count"`
	AvgDuration  float64  `json:"avg_duration_sec"`
	TotalCalories float64 `json:"total_calories"`
	AvgHeartRate *float64 `json:"avg_heart_rate,omitempty"`
}

// StrengthVolumeSummary holds aggregated strength training stats for a period.
type StrengthVolumeSummary struct {
	WorkingSets    int     `json:"working_sets"`
	TotalReps      int     `json:"total_reps"`
	TonnageKg      float64 `json:"tonnage_kg"`
	Sessions       int     `json:"sessions"`
	AvgSetsPerSession float64 `json:"avg_sets_per_session"`
}

// TrainingSummaryPeriod holds combined workout + strength data for one time period.
type TrainingSummaryPeriod struct {
	Period    string                     `json:"period"`
	Workouts  []WorkoutTypePeriodSummary `json:"workouts"`
	Strength  *StrengthVolumeSummary     `json:"strength,omitempty"`
}

// GetTrainingSummary returns aggregated workout and strength volume stats per period.
func (db *DB) GetTrainingSummary(ctx context.Context, start, end time.Time, bucket string, userID int) ([]TrainingSummaryPeriod, error) {
	// Query 1: Workout stats grouped by period + type
	workoutRows, err := db.Pool.Query(ctx,
		`SELECT date_trunc($1, start_time)::date AS period,
		        name,
		        COUNT(*)::int,
		        AVG(duration_sec),
		        COALESCE(SUM(active_energy_burned), 0),
		        AVG(avg_heart_rate)
		 FROM workouts
		 WHERE start_time >= $2 AND start_time < $3 AND user_id = $4
		 GROUP BY period, name
		 ORDER BY period DESC, COUNT(*) DESC`,
		truncInterval(bucket), start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying workout summary: %w", err)
	}
	defer workoutRows.Close()

	// Build map of period -> workout summaries
	periodMap := make(map[string]*TrainingSummaryPeriod)
	var periodOrder []string

	for workoutRows.Next() {
		var periodTime time.Time
		var ws WorkoutTypePeriodSummary
		if err := workoutRows.Scan(&periodTime, &ws.Type, &ws.Count, &ws.AvgDuration, &ws.TotalCalories, &ws.AvgHeartRate); err != nil {
			return nil, fmt.Errorf("scanning workout summary: %w", err)
		}
		key := periodTime.Format("2006-01-02")
		if _, ok := periodMap[key]; !ok {
			periodMap[key] = &TrainingSummaryPeriod{Period: key}
			periodOrder = append(periodOrder, key)
		}
		periodMap[key].Workouts = append(periodMap[key].Workouts, ws)
	}
	if err := workoutRows.Err(); err != nil {
		return nil, err
	}

	// Query 2: Strength set volume grouped by period
	strengthRows, err := db.Pool.Query(ctx,
		`SELECT date_trunc($1, session_date)::date AS period,
		        COUNT(*) FILTER (WHERE NOT is_warmup)::int AS working_sets,
		        COALESCE(SUM(reps) FILTER (WHERE NOT is_warmup), 0)::int AS total_reps,
		        COALESCE(SUM(weight_kg * reps) FILTER (WHERE NOT is_warmup), 0) AS tonnage,
		        COUNT(DISTINCT session_date)::int AS sessions
		 FROM workout_sets
		 WHERE session_date >= $2 AND session_date < $3 AND user_id = $4
		 GROUP BY period
		 ORDER BY period DESC`,
		truncInterval(bucket), start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying strength summary: %w", err)
	}
	defer strengthRows.Close()

	for strengthRows.Next() {
		var periodTime time.Time
		var sv StrengthVolumeSummary
		if err := strengthRows.Scan(&periodTime, &sv.WorkingSets, &sv.TotalReps, &sv.TonnageKg, &sv.Sessions); err != nil {
			return nil, fmt.Errorf("scanning strength summary: %w", err)
		}
		if sv.Sessions > 0 {
			sv.AvgSetsPerSession = float64(sv.WorkingSets) / float64(sv.Sessions)
		}
		key := periodTime.Format("2006-01-02")
		if _, ok := periodMap[key]; !ok {
			periodMap[key] = &TrainingSummaryPeriod{Period: key}
			periodOrder = append(periodOrder, key)
		}
		periodMap[key].Strength = &sv
	}
	if err := strengthRows.Err(); err != nil {
		return nil, err
	}

	// Assemble result in order
	result := make([]TrainingSummaryPeriod, 0, len(periodOrder))
	for _, key := range periodOrder {
		result = append(result, *periodMap[key])
	}
	return result, nil
}

// truncInterval converts bucket strings like "1 month" to the interval name
// that date_trunc expects (e.g. "month", "week").
func truncInterval(bucket string) string {
	switch bucket {
	case "1 week":
		return "week"
	case "1 month":
		return "month"
	default:
		return "month"
	}
}
