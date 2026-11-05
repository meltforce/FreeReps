package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/claude/freereps/internal/models"
)

// workoutSetColumns is the insert column list; its length drives the parameter
// arithmetic in InsertWorkoutSets. effort_rir is absent on purpose — the
// database generates it from rir and rpe.
var workoutSetColumns = []string{
	"user_id", "source", "external_id", "routine_id",
	"session_name", "session_date", "session_end", "session_duration",
	"exercise_number", "exercise_name", "exercise_template_id", "exercise_notes",
	"equipment", "target_reps", "is_warmup", "set_type", "set_number", "superset_id",
	"weight_kg", "is_bodyweight_plus", "reps", "rir", "rpe",
	"distance_m", "duration_sec", "custom_metric", "source_exercise_id",
}

// DeleteWorkoutSets removes all sets for a given session date and user, enabling clean re-imports.
func (db *DB) DeleteWorkoutSets(ctx context.Context, sessionDate time.Time, userID int) error {
	_, err := db.Pool.Exec(ctx,
		`DELETE FROM workout_sets WHERE user_id = $1 AND session_date = $2`,
		userID, sessionDate)
	return err
}

// DeleteWorkoutSetsByExternalID removes every set belonging to one source-side
// workout. The Hevy sync calls this before re-inserting an updated workout:
// ON CONFLICT DO NOTHING alone would silently discard a correction made in the
// app, and a deleted set would survive forever.
func (db *DB) DeleteWorkoutSetsByExternalID(ctx context.Context, userID int, source, externalID string) (int64, error) {
	if externalID == "" {
		return 0, fmt.Errorf("external_id must not be empty")
	}
	tag, err := db.Pool.Exec(ctx,
		`DELETE FROM workout_sets WHERE user_id = $1 AND source = $2 AND external_id = $3`,
		userID, source, externalID)
	if err != nil {
		return 0, fmt.Errorf("deleting workout sets by external id: %w", err)
	}
	return tag.RowsAffected(), nil
}

// insertWorkoutSetsBatchSize caps how many rows go into one INSERT. The
// extended protocol allows 65535 parameters per statement and a row costs
// len(workoutSetColumns) of them, so a full Alpha Progression export — around
// 4000 rows against 27 columns — exceeds the limit in a single statement and
// the whole import fails with "extended protocol limited to 65535 parameters".
// Same failure that f55a5a3 fixed for workout_routes; workout_sets crossed the
// limit later, when migration 000020 widened the row for Hevy.
const insertWorkoutSetsBatchSize = 2000

// InsertWorkoutSets batch-inserts set data. Returns count inserted.
func (db *DB) InsertWorkoutSets(ctx context.Context, rows []models.WorkoutSetRow) (int64, error) {
	var total int64
	for start := 0; start < len(rows); start += insertWorkoutSetsBatchSize {
		end := min(start+insertWorkoutSetsBatchSize, len(rows))
		n, err := db.insertWorkoutSetBatch(ctx, rows[start:end])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (db *DB) insertWorkoutSetBatch(ctx context.Context, rows []models.WorkoutSetRow) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	n := len(workoutSetColumns)
	query := `INSERT INTO workout_sets (` + strings.Join(workoutSetColumns, ", ") + `) VALUES `
	args := make([]any, 0, len(rows)*n)
	valueStrings := make([]string, 0, len(rows))

	for i, r := range rows {
		base := i * n
		placeholders := make([]string, n)
		for j := range placeholders {
			placeholders[j] = fmt.Sprintf("$%d", base+j+1)
		}
		valueStrings = append(valueStrings, "("+strings.Join(placeholders, ",")+")")

		source := r.Source
		if source == "" {
			source = "Alpha Progression"
		}
		setType := r.SetType
		if setType == "" {
			setType = "normal"
		}

		args = append(args,
			r.UserID, source, r.ExternalID, r.RoutineID,
			r.SessionName, r.SessionDate, r.SessionEnd, r.SessionDuration,
			r.ExerciseNumber, r.ExerciseName, r.ExerciseTemplateID, r.ExerciseNotes,
			r.Equipment, r.TargetReps, r.IsWarmup, setType, r.SetNumber, r.SupersetID,
			r.WeightKg, r.IsBodyweightPlus, r.Reps, r.RIR, r.RPE,
			r.DistanceM, r.DurationSec, r.CustomMetric, r.SourceExerciseID,
		)
	}

	query += strings.Join(valueStrings, ",") + " ON CONFLICT DO NOTHING"

	tag, err := db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("inserting workout sets: %w", err)
	}
	return tag.RowsAffected(), nil
}

// QueryWorkoutSets retrieves workout sets in a date range, optionally filtered by exercise name.
func (db *DB) QueryWorkoutSets(ctx context.Context, start, end time.Time, userID int, exerciseFilter string) ([]models.WorkoutSetRow, error) {
	// The muscle group comes from the exercise catalog: Hevy rows reference it
	// directly, Alpha and openGym rows reach it through exercise_name_map — keyed
	// by the source's exercise id where it has one, by the name otherwise.
	query := `SELECT ws.user_id, ws.source, ws.external_id, ws.routine_id,
		 ws.session_name, ws.session_date, ws.session_end, ws.session_duration,
		 ws.exercise_number, ws.exercise_name, ws.exercise_template_id, ws.exercise_notes,
		 COALESCE(t.primary_muscle_group, ''),
		 ws.equipment, ws.target_reps, ws.is_warmup, ws.set_type, ws.set_number, ws.superset_id,
		 ws.weight_kg, ws.is_bodyweight_plus, ws.reps, ws.rir, ws.rpe, ws.effort_rir,
		 ws.distance_m, ws.duration_sec, ws.custom_metric
		 FROM workout_sets ws
		 LEFT JOIN exercise_name_map m
		        ON m.source = ws.source
		       AND m.source_key = COALESCE(NULLIF(ws.source_exercise_id, ''), ws.exercise_name)
		 LEFT JOIN exercise_templates t
		        ON t.id = COALESCE(NULLIF(ws.exercise_template_id, ''), m.exercise_template_id)
		 WHERE ws.session_date >= $1 AND ws.session_date < $2 AND ws.user_id = $3`
	args := []any{start, end, userID}
	if exerciseFilter != "" {
		query += ` AND ws.exercise_name ILIKE '%' || $4 || '%'`
		args = append(args, exerciseFilter)
	}
	query += ` ORDER BY ws.session_date DESC, ws.exercise_number ASC, ws.is_warmup DESC, ws.set_number ASC`
	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying workout sets: %w", err)
	}
	defer rows.Close()

	var result []models.WorkoutSetRow
	for rows.Next() {
		var r models.WorkoutSetRow
		if err := rows.Scan(&r.UserID, &r.Source, &r.ExternalID, &r.RoutineID,
			&r.SessionName, &r.SessionDate, &r.SessionEnd, &r.SessionDuration,
			&r.ExerciseNumber, &r.ExerciseName, &r.ExerciseTemplateID, &r.ExerciseNotes,
			&r.PrimaryMuscleGroup,
			&r.Equipment, &r.TargetReps, &r.IsWarmup, &r.SetType, &r.SetNumber, &r.SupersetID,
			&r.WeightKg, &r.IsBodyweightPlus, &r.Reps, &r.RIR, &r.RPE, &r.EffortRIR,
			&r.DistanceM, &r.DurationSec, &r.CustomMetric); err != nil {
			return nil, fmt.Errorf("scanning workout set: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// SetSessionInfo summarizes a distinct strength training session in workout_sets.
type SetSessionInfo struct {
	Source          string
	ExternalID      string
	SessionName     string
	SessionDate     time.Time
	SessionEnd      *time.Time
	SessionDuration string
}

// QuerySetSessions returns one row per distinct strength session in a time range,
// across all sources. QueryWorkoutsMerged turns these into workout entries.
func (db *DB) QuerySetSessions(ctx context.Context, start, end time.Time, userID int) ([]SetSessionInfo, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT DISTINCT source, external_id, session_name, session_date, session_end, session_duration
		 FROM workout_sets
		 WHERE session_date >= $1 AND session_date < $2 AND user_id = $3
		 ORDER BY session_date DESC`,
		start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying set sessions: %w", err)
	}
	defer rows.Close()

	var result []SetSessionInfo
	for rows.Next() {
		var s SetSessionInfo
		if err := rows.Scan(&s.Source, &s.ExternalID, &s.SessionName, &s.SessionDate,
			&s.SessionEnd, &s.SessionDuration); err != nil {
			return nil, fmt.Errorf("scanning set session: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}
