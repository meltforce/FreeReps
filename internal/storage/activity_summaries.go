package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/claude/freereps/internal/models"
)

// InsertActivitySummaries batch-inserts activity summary rows. Returns count inserted.
// Uses ON CONFLICT DO NOTHING on (user_id, date) composite PK.
func (db *DB) InsertActivitySummaries(ctx context.Context, rows []models.ActivitySummaryRow) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	query := `INSERT INTO activity_summaries (user_id, date, active_energy, active_energy_goal, exercise_time, exercise_time_goal, stand_hours, stand_hours_goal) VALUES `
	args := make([]any, 0, len(rows)*8)
	valueStrings := make([]string, 0, len(rows))

	for i, r := range rows {
		base := i * 8
		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8,
		))
		args = append(args, r.UserID, r.Date, r.ActiveEnergy, r.ActiveEnergyGoal,
			r.ExerciseTime, r.ExerciseTimeGoal, r.StandHours, r.StandHoursGoal)
	}

	query += strings.Join(valueStrings, ",") + " ON CONFLICT DO NOTHING"

	tag, err := db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("inserting activity summaries: %w", err)
	}
	return tag.RowsAffected(), nil
}

// QueryActivitySummaries retrieves activity summaries in a date range for a user.
func (db *DB) QueryActivitySummaries(ctx context.Context, start, end time.Time, userID int) ([]models.ActivitySummaryRow, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT user_id, date, active_energy, active_energy_goal, exercise_time, exercise_time_goal, stand_hours, stand_hours_goal
		 FROM activity_summaries
		 WHERE date >= $1 AND date < $2 AND user_id = $3
		 ORDER BY date DESC`,
		start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying activity summaries: %w", err)
	}
	defer rows.Close()

	var result []models.ActivitySummaryRow
	for rows.Next() {
		var r models.ActivitySummaryRow
		if err := rows.Scan(&r.UserID, &r.Date, &r.ActiveEnergy, &r.ActiveEnergyGoal,
			&r.ExerciseTime, &r.ExerciseTimeGoal, &r.StandHours, &r.StandHoursGoal); err != nil {
			return nil, fmt.Errorf("scanning activity summary: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
