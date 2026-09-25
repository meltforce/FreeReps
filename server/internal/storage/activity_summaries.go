package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/claude/freereps/internal/models"
)

// InsertActivitySummaries batch-upserts activity summary rows and returns the
// number inserted or changed. A day's summary grows until the day ends and the
// app sends it on every sync; under ON CONFLICT DO NOTHING the first delivery
// stayed, which left 2026-09-18 at 6 kcal. A changed summary now replaces the
// stored one; an identical one writes nothing. Rows repeating a date within one
// call keep the last, since DO UPDATE cannot touch one row twice.
func (db *DB) InsertActivitySummaries(ctx context.Context, rows []models.ActivitySummaryRow) (int64, error) {
	rows = dedupeActivitySummaryRows(rows)
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

	query += strings.Join(valueStrings, ",") + `
ON CONFLICT (user_id, date) DO UPDATE SET
	active_energy = EXCLUDED.active_energy, active_energy_goal = EXCLUDED.active_energy_goal,
	exercise_time = EXCLUDED.exercise_time, exercise_time_goal = EXCLUDED.exercise_time_goal,
	stand_hours = EXCLUDED.stand_hours, stand_hours_goal = EXCLUDED.stand_hours_goal
WHERE (activity_summaries.active_energy, activity_summaries.active_energy_goal,
       activity_summaries.exercise_time, activity_summaries.exercise_time_goal,
       activity_summaries.stand_hours, activity_summaries.stand_hours_goal)
	IS DISTINCT FROM
      (EXCLUDED.active_energy, EXCLUDED.active_energy_goal, EXCLUDED.exercise_time,
       EXCLUDED.exercise_time_goal, EXCLUDED.stand_hours, EXCLUDED.stand_hours_goal)`

	tag, err := db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("inserting activity summaries: %w", err)
	}
	return tag.RowsAffected(), nil
}

// dedupeActivitySummaryRows keeps the last row per (user, day).
func dedupeActivitySummaryRows(rows []models.ActivitySummaryRow) []models.ActivitySummaryRow {
	type key struct {
		user int
		date time.Time
	}
	last := make(map[key]int, len(rows))
	for i, r := range rows {
		last[key{r.UserID, r.Date.UTC()}] = i
	}
	if len(last) == len(rows) {
		return rows
	}
	out := make([]models.ActivitySummaryRow, 0, len(last))
	for i, r := range rows {
		if last[key{r.UserID, r.Date.UTC()}] == i {
			out = append(out, r)
		}
	}
	return out
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
