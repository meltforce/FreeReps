package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/claude/freereps/internal/models"
)

// InsertWorkoutSets batch-inserts Alpha Progression set data. Returns count inserted.
func (db *DB) InsertWorkoutSets(ctx context.Context, rows []models.WorkoutSetRow) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	query := `INSERT INTO workout_sets (user_id, session_name, session_date, session_duration,
		exercise_number, exercise_name, equipment, target_reps, is_warmup, set_number,
		weight_kg, is_bodyweight_plus, reps, rir) VALUES `
	args := make([]any, 0, len(rows)*14)
	valueStrings := make([]string, 0, len(rows))

	for i, r := range rows {
		base := i * 14
		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7,
			base+8, base+9, base+10, base+11, base+12, base+13, base+14,
		))
		args = append(args, r.UserID, r.SessionName, r.SessionDate, r.SessionDuration,
			r.ExerciseNumber, r.ExerciseName, r.Equipment, r.TargetReps,
			r.IsWarmup, r.SetNumber, r.WeightKg, r.IsBodyweightPlus, r.Reps, r.RIR)
	}

	query += strings.Join(valueStrings, ",") + " ON CONFLICT DO NOTHING"

	tag, err := db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("inserting workout sets: %w", err)
	}
	return tag.RowsAffected(), nil
}

// QueryWorkoutSets retrieves workout sets in a date range.
func (db *DB) QueryWorkoutSets(ctx context.Context, start, end time.Time, userID int) ([]models.WorkoutSetRow, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT user_id, session_name, session_date, session_duration,
		 exercise_number, exercise_name, equipment, target_reps,
		 is_warmup, set_number, weight_kg, is_bodyweight_plus, reps, rir
		 FROM workout_sets
		 WHERE session_date >= $1 AND session_date < $2 AND user_id = $3
		 ORDER BY session_date DESC, exercise_number ASC, is_warmup DESC, set_number ASC`,
		start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying workout sets: %w", err)
	}
	defer rows.Close()

	var result []models.WorkoutSetRow
	for rows.Next() {
		var r models.WorkoutSetRow
		if err := rows.Scan(&r.UserID, &r.SessionName, &r.SessionDate, &r.SessionDuration,
			&r.ExerciseNumber, &r.ExerciseName, &r.Equipment, &r.TargetReps,
			&r.IsWarmup, &r.SetNumber, &r.WeightKg, &r.IsBodyweightPlus, &r.Reps, &r.RIR); err != nil {
			return nil, fmt.Errorf("scanning workout set: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
