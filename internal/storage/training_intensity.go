package storage

import (
	"context"
	"fmt"
	"time"
)

// RIRBand holds the count and percentage of sets in a specific RIR range.
type RIRBand struct {
	Band     string  `json:"band"`
	RIRRange string  `json:"rir_range"`
	Sets     int     `json:"sets"`
	Pct      float64 `json:"pct"`
}

// ExerciseSummary holds aggregated stats for a single exercise.
type ExerciseSummary struct {
	Name       string   `json:"name"`
	TotalSets  int      `json:"total_sets"`
	TotalReps  int      `json:"total_reps"`
	TonnageKg  float64  `json:"tonnage_kg"`
	MaxWeight  float64  `json:"max_weight_kg"`
	AvgRIR     *float64 `json:"avg_rir,omitempty"`
}

// ExerciseProgression holds one session's data for a specific exercise.
type ExerciseProgression struct {
	Date           string   `json:"date"`
	MaxWeight      float64  `json:"max_weight_kg"`
	SessionTonnage float64  `json:"session_tonnage_kg"`
	Sets           int      `json:"sets"`
	AvgRIR         *float64 `json:"avg_rir,omitempty"`
}

// TrainingIntensityResult holds the complete intensity analysis.
type TrainingIntensityResult struct {
	RIRDistribution []RIRBand             `json:"rir_distribution"`
	FailureRatePct  float64               `json:"failure_rate_pct"`
	TotalSets       int                   `json:"total_sets"`
	TrackedSets     int                   `json:"tracked_sets"`
	Exercises       []ExerciseSummary     `json:"exercises"`
	Progression     []ExerciseProgression `json:"progression,omitempty"`
}

// GetTrainingIntensity returns RIR distribution, failure rate, per-exercise stats,
// and optional exercise progression for strength training.
// RIR value of -1 is treated as untracked (Alpha Progression sentinel).
func (db *DB) GetTrainingIntensity(ctx context.Context, start, end time.Time, userID int, exerciseFilter string) (*TrainingIntensityResult, error) {
	result := &TrainingIntensityResult{}

	// Query 1: RIR distribution
	rirRows, err := db.Pool.Query(ctx,
		`SELECT band, rir_range, sets FROM (
			SELECT
				CASE
					WHEN rir = -1 THEN 'untracked'
					WHEN rir <= 0 THEN 'failure'
					WHEN rir <= 1 THEN 'near_failure'
					WHEN rir <= 2 THEN 'moderate'
					WHEN rir <= 3 THEN 'easy'
					ELSE 'very_easy'
				END AS band,
				CASE
					WHEN rir = -1 THEN 'untracked'
					WHEN rir <= 0 THEN '0'
					WHEN rir <= 1 THEN '0.5-1'
					WHEN rir <= 2 THEN '1.5-2'
					WHEN rir <= 3 THEN '2.5-3'
					ELSE '>3'
				END AS rir_range,
				COUNT(*)::int AS sets
			FROM workout_sets
			WHERE session_date >= $1 AND session_date < $2
				AND user_id = $3
				AND NOT is_warmup
			GROUP BY band, rir_range
		) sub
		ORDER BY CASE band
			WHEN 'failure' THEN 1
			WHEN 'near_failure' THEN 2
			WHEN 'moderate' THEN 3
			WHEN 'easy' THEN 4
			WHEN 'very_easy' THEN 5
			WHEN 'untracked' THEN 6
		END`,
		start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying RIR distribution: %w", err)
	}
	defer rirRows.Close()

	var totalSets, trackedSets, failureSets int
	for rirRows.Next() {
		var b RIRBand
		if err := rirRows.Scan(&b.Band, &b.RIRRange, &b.Sets); err != nil {
			return nil, fmt.Errorf("scanning RIR band: %w", err)
		}
		totalSets += b.Sets
		if b.Band != "untracked" {
			trackedSets += b.Sets
		}
		if b.Band == "failure" || b.Band == "near_failure" {
			failureSets += b.Sets
		}
		result.RIRDistribution = append(result.RIRDistribution, b)
	}
	if err := rirRows.Err(); err != nil {
		return nil, err
	}

	result.TotalSets = totalSets
	result.TrackedSets = trackedSets

	// Compute percentages
	for i := range result.RIRDistribution {
		if totalSets > 0 {
			result.RIRDistribution[i].Pct = float64(result.RIRDistribution[i].Sets) / float64(totalSets) * 100
		}
	}

	if trackedSets > 0 {
		result.FailureRatePct = float64(failureSets) / float64(trackedSets) * 100
	}

	// Query 2: Per-exercise summary
	exRows, err := db.Pool.Query(ctx,
		`SELECT exercise_name,
		        COUNT(*)::int,
		        COALESCE(SUM(reps), 0)::int,
		        COALESCE(SUM(weight_kg * reps), 0),
		        COALESCE(MAX(weight_kg), 0),
		        AVG(NULLIF(rir, -1))
		 FROM workout_sets
		 WHERE session_date >= $1 AND session_date < $2
		   AND user_id = $3
		   AND NOT is_warmup
		 GROUP BY exercise_name
		 ORDER BY SUM(weight_kg * reps) DESC`,
		start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying exercise summary: %w", err)
	}
	defer exRows.Close()

	for exRows.Next() {
		var e ExerciseSummary
		if err := exRows.Scan(&e.Name, &e.TotalSets, &e.TotalReps, &e.TonnageKg, &e.MaxWeight, &e.AvgRIR); err != nil {
			return nil, fmt.Errorf("scanning exercise summary: %w", err)
		}
		result.Exercises = append(result.Exercises, e)
	}
	if err := exRows.Err(); err != nil {
		return nil, err
	}

	// Query 3: Exercise progression (only when filter is set)
	if exerciseFilter != "" {
		progRows, err := db.Pool.Query(ctx,
			`SELECT session_date,
			        COALESCE(MAX(weight_kg), 0),
			        COALESCE(SUM(weight_kg * reps), 0),
			        COUNT(*)::int,
			        AVG(NULLIF(rir, -1))
			 FROM workout_sets
			 WHERE session_date >= $1 AND session_date < $2
			   AND user_id = $3
			   AND exercise_name ILIKE '%' || $4 || '%'
			   AND NOT is_warmup
			 GROUP BY session_date
			 ORDER BY session_date ASC`,
			start, end, userID, exerciseFilter)
		if err != nil {
			return nil, fmt.Errorf("querying exercise progression: %w", err)
		}
		defer progRows.Close()

		for progRows.Next() {
			var p ExerciseProgression
			var d time.Time
			if err := progRows.Scan(&d, &p.MaxWeight, &p.SessionTonnage, &p.Sets, &p.AvgRIR); err != nil {
				return nil, fmt.Errorf("scanning exercise progression: %w", err)
			}
			p.Date = d.Format("2006-01-02")
			result.Progression = append(result.Progression, p)
		}
		if err := progRows.Err(); err != nil {
			return nil, err
		}
	}

	return result, nil
}
