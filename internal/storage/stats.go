package storage

import (
	"context"
	"fmt"
	"time"
)

// DataStats holds aggregate statistics about all stored data.
type DataStats struct {
	TotalMetricRows  int64           `json:"total_metric_rows"`
	TotalWorkouts    int64           `json:"total_workouts"`
	TotalSleepNights int64           `json:"total_sleep_nights"`
	TotalSets        int64           `json:"total_sets"`
	EarliestData     *time.Time      `json:"earliest_data"`
	LatestData       *time.Time      `json:"latest_data"`
	WorkoutsByType   []WorkoutTypeStat `json:"workouts_by_type"`
}

// WorkoutTypeStat holds summary stats for a single workout type.
type WorkoutTypeStat struct {
	Name          string   `json:"name"`
	Count         int64    `json:"count"`
	TotalDuration float64  `json:"total_duration_sec"`
	TotalDistance  *float64 `json:"total_distance,omitempty"`
}

// GetDataStats returns aggregate statistics for a user's stored data.
func (db *DB) GetDataStats(ctx context.Context, userID int) (*DataStats, error) {
	stats := &DataStats{}

	// Total metric rows
	err := db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM health_metrics WHERE user_id = $1`, userID,
	).Scan(&stats.TotalMetricRows)
	if err != nil {
		return nil, fmt.Errorf("counting metrics: %w", err)
	}

	// Total workouts
	err = db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM workouts WHERE user_id = $1`, userID,
	).Scan(&stats.TotalWorkouts)
	if err != nil {
		return nil, fmt.Errorf("counting workouts: %w", err)
	}

	// Total sleep nights
	err = db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM sleep_sessions WHERE user_id = $1`, userID,
	).Scan(&stats.TotalSleepNights)
	if err != nil {
		return nil, fmt.Errorf("counting sleep sessions: %w", err)
	}

	// Total sets
	err = db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM workout_sets WHERE user_id = $1`, userID,
	).Scan(&stats.TotalSets)
	if err != nil {
		return nil, fmt.Errorf("counting sets: %w", err)
	}

	// Date range (earliest/latest across metrics and workouts)
	err = db.Pool.QueryRow(ctx,
		`SELECT MIN(t), MAX(t) FROM (
			SELECT MIN(time) AS t FROM health_metrics WHERE user_id = $1
			UNION ALL
			SELECT MIN(start_time) FROM workouts WHERE user_id = $1
			UNION ALL
			SELECT MAX(time) FROM health_metrics WHERE user_id = $1
			UNION ALL
			SELECT MAX(start_time) FROM workouts WHERE user_id = $1
		) sub`, userID,
	).Scan(&stats.EarliestData, &stats.LatestData)
	if err != nil {
		return nil, fmt.Errorf("querying date range: %w", err)
	}

	// Workouts by type
	rows, err := db.Pool.Query(ctx,
		`SELECT name, COUNT(*), COALESCE(SUM(duration_sec), 0), SUM(distance)
		 FROM workouts
		 WHERE user_id = $1
		 GROUP BY name
		 ORDER BY COUNT(*) DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying workouts by type: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var s WorkoutTypeStat
		if err := rows.Scan(&s.Name, &s.Count, &s.TotalDuration, &s.TotalDistance); err != nil {
			return nil, fmt.Errorf("scanning workout type stat: %w", err)
		}
		stats.WorkoutsByType = append(stats.WorkoutsByType, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}
