package importer

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/claude/freereps/internal/models"
	"github.com/claude/freereps/internal/storage"
)

// CorrelateWorkoutHR populates the workout_heart_rate table by finding heart_rate
// data points that overlap each workout's time range. This is needed because .hae
// file exports store workout HR data separately in HealthMetrics/heart_rate/ files
// rather than inline with the workout (unlike the REST API).
//
// Only workouts that have zero existing HR data are processed (idempotent).
func CorrelateWorkoutHR(ctx context.Context, db *storage.DB, log *slog.Logger) (int64, error) {
	// Find workouts that have no HR data yet
	rows, err := db.Pool.Query(ctx,
		`SELECT w.id, w.user_id, w.start_time, w.end_time
		 FROM workouts w
		 LEFT JOIN workout_heart_rate whr ON w.id = whr.workout_id
		 WHERE whr.workout_id IS NULL`)
	if err != nil {
		return 0, fmt.Errorf("querying workouts without HR: %w", err)
	}
	defer rows.Close()

	type workoutInfo struct {
		row models.WorkoutRow
	}
	var workouts []workoutInfo
	for rows.Next() {
		var w workoutInfo
		if err := rows.Scan(&w.row.ID, &w.row.UserID, &w.row.StartTime, &w.row.EndTime); err != nil {
			return 0, fmt.Errorf("scanning workout: %w", err)
		}
		workouts = append(workouts, w)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	if len(workouts) == 0 {
		return 0, nil
	}

	log.Info("correlating HR data for workouts", "count", len(workouts))

	var totalInserted int64
	for _, w := range workouts {
		// Query heart_rate metrics that fall within the workout time range
		hrMetrics, err := db.QueryHealthMetrics(ctx, "heart_rate", w.row.StartTime, w.row.EndTime, w.row.UserID)
		if err != nil {
			return totalInserted, fmt.Errorf("querying HR for workout %s: %w", w.row.ID, err)
		}

		if len(hrMetrics) == 0 {
			continue
		}

		// Convert health metric rows to workout HR rows
		hrRows := make([]models.WorkoutHRRow, len(hrMetrics))
		for i, hm := range hrMetrics {
			hrRows[i] = models.WorkoutHRRow{
				Time:      hm.Time,
				WorkoutID: w.row.ID,
				UserID:    w.row.UserID,
				MinBPM:    hm.MinVal,
				AvgBPM:    hm.AvgVal,
				MaxBPM:    hm.MaxVal,
				Source:    hm.Source,
			}
		}

		// Batch insert (7 params per row)
		const batchSize = 9000
		for i := 0; i < len(hrRows); i += batchSize {
			end := i + batchSize
			if end > len(hrRows) {
				end = len(hrRows)
			}
			inserted, err := db.InsertWorkoutHeartRate(ctx, hrRows[i:end])
			if err != nil {
				return totalInserted, fmt.Errorf("inserting correlated HR for workout %s: %w", w.row.ID, err)
			}
			totalInserted += inserted
		}
	}

	return totalInserted, nil
}
