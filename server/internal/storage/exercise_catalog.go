package storage

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ExerciseTemplate is a row in the exercise catalog.
type ExerciseTemplate struct {
	ID                    string    `json:"id"`
	Title                 string    `json:"title"`
	ExerciseType          string    `json:"exercise_type"`
	PrimaryMuscleGroup    string    `json:"primary_muscle_group"`
	SecondaryMuscleGroups []string  `json:"secondary_muscle_groups"`
	EquipmentCategory     string    `json:"equipment_category"`
	IsCustom              bool      `json:"is_custom"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// UpsertExerciseTemplates writes the catalog, replacing entries that changed.
// Returns the number of rows written.
func (db *DB) UpsertExerciseTemplates(ctx context.Context, rows []ExerciseTemplate) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	const cols = 7
	query := `INSERT INTO exercise_templates
		(id, title, exercise_type, primary_muscle_group, secondary_muscle_groups,
		 equipment_category, is_custom) VALUES `
	args := make([]any, 0, len(rows)*cols)
	values := make([]string, 0, len(rows))

	for i, r := range rows {
		base := i * cols
		values = append(values, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7))
		secondary := r.SecondaryMuscleGroups
		if secondary == nil {
			secondary = []string{}
		}
		args = append(args, r.ID, r.Title, r.ExerciseType, r.PrimaryMuscleGroup,
			secondary, r.EquipmentCategory, r.IsCustom)
	}

	query += strings.Join(values, ",") + `
		ON CONFLICT (id) DO UPDATE SET
		  title = EXCLUDED.title,
		  exercise_type = EXCLUDED.exercise_type,
		  primary_muscle_group = EXCLUDED.primary_muscle_group,
		  secondary_muscle_groups = EXCLUDED.secondary_muscle_groups,
		  equipment_category = EXCLUDED.equipment_category,
		  is_custom = EXCLUDED.is_custom,
		  updated_at = NOW()`

	tag, err := db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("upserting exercise templates: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ListExerciseTemplates returns the full catalog, ordered by title.
func (db *DB) ListExerciseTemplates(ctx context.Context) ([]ExerciseTemplate, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, title, exercise_type, primary_muscle_group,
		        secondary_muscle_groups, equipment_category, is_custom, updated_at
		 FROM exercise_templates ORDER BY title`)
	if err != nil {
		return nil, fmt.Errorf("listing exercise templates: %w", err)
	}
	defer rows.Close()

	var result []ExerciseTemplate
	for rows.Next() {
		var t ExerciseTemplate
		if err := rows.Scan(&t.ID, &t.Title, &t.ExerciseType, &t.PrimaryMuscleGroup,
			&t.SecondaryMuscleGroups, &t.EquipmentCategory, &t.IsCustom, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning exercise template: %w", err)
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// CountExerciseTemplates returns how many entries the catalog holds.
func (db *DB) CountExerciseTemplates(ctx context.Context) (int, error) {
	var n int
	if err := db.Pool.QueryRow(ctx, `SELECT count(*) FROM exercise_templates`).Scan(&n); err != nil {
		return 0, fmt.Errorf("counting exercise templates: %w", err)
	}
	return n, nil
}

// UnmappedExerciseName is an exercise name from a source without template ids
// that has no catalog entry, together with how much data hangs off it.
//
// SourceKey is the value a new exercise_name_map row needs as its key: the
// source's exercise id where the source has one (openGym), the name otherwise.
type UnmappedExerciseName struct {
	Source       string `json:"source"`
	SourceKey    string `json:"source_key"`
	ExerciseName string `json:"exercise_name"`
	Sets         int    `json:"sets"`
}

// ListUnmappedExerciseNames returns the exercise names that carry neither a
// template id on the set nor an entry in exercise_name_map, ordered by how many
// sets they account for. This is the work list for the mapping, and afterwards
// the measure of what any volume figure leaves out.
func (db *DB) ListUnmappedExerciseNames(ctx context.Context, userID int) ([]UnmappedExerciseName, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT ws.source,
		        COALESCE(NULLIF(ws.source_exercise_id, ''), ws.exercise_name) AS source_key,
		        ws.exercise_name, COUNT(*)::int AS sets
		 FROM workout_sets ws
		 LEFT JOIN exercise_name_map m
		   ON m.source = ws.source
		  AND m.source_key = COALESCE(NULLIF(ws.source_exercise_id, ''), ws.exercise_name)
		 WHERE ws.user_id = $1
		   AND COALESCE(NULLIF(ws.exercise_template_id, ''), m.exercise_template_id) IS NULL
		 GROUP BY 1, 2, 3
		 ORDER BY sets DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing unmapped exercise names: %w", err)
	}
	defer rows.Close()

	var result []UnmappedExerciseName
	for rows.Next() {
		var u UnmappedExerciseName
		if err := rows.Scan(&u.Source, &u.SourceKey, &u.ExerciseName, &u.Sets); err != nil {
			return nil, fmt.Errorf("scanning unmapped exercise name: %w", err)
		}
		result = append(result, u)
	}
	return result, rows.Err()
}
