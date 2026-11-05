package storage

import (
	"context"
	"fmt"
	"math"
	"time"
)

// secondaryMuscleWeight is what one set counts towards a muscle that assists the
// movement rather than driving it.
//
// The figure is a convention, not a measurement — no data in this system says
// how much of a bench press the triceps carry. Both the primary count and the
// weighted count are reported so the assumption stays visible instead of
// disappearing into a single number.
const secondaryMuscleWeight = 0.5

// MuscleVolume holds weekly volume for one muscle group.
type MuscleVolume struct {
	Muscle string `json:"muscle"`
	// PrimarySets counts sets whose exercise targets this muscle directly.
	PrimarySets int `json:"primary_sets"`
	// WeightedSets adds assisting muscles at secondaryMuscleWeight.
	WeightedSets float64 `json:"weighted_sets"`
	// Sessions is how many separate training days touched this muscle.
	Sessions  int     `json:"sessions"`
	TonnageKg float64 `json:"tonnage_kg"`
	// ApproximatePct is the share of WeightedSets coming from exercise names
	// mapped onto a near equivalent rather than an exact one. High values mean
	// the split between neighbouring muscle groups is soft, not that the total
	// is wrong.
	ApproximatePct float64 `json:"approximate_pct"`
}

// TrainingVolumePeriod holds the volume breakdown for one period.
type TrainingVolumePeriod struct {
	Period  string         `json:"period"`
	Muscles []MuscleVolume `json:"muscles"`
	// UnmappedSets counts working sets whose exercise resolves to no catalog
	// entry. They appear in no muscle group, so a volume figure without this
	// number is a statement about an unknown fraction of the training.
	UnmappedSets  int      `json:"unmapped_sets"`
	UnmappedNames []string `json:"unmapped_names,omitempty"`
}

// resolvedSetsCTE joins workout_sets to the exercise catalog. Hevy rows carry a
// template id directly; Alpha and openGym rows reach it through
// exercise_name_map, keyed by the source's exercise id where it has one.
const resolvedSetsCTE = `
	resolved AS (
		SELECT ws.session_date, ws.weight_kg, ws.reps, ws.exercise_name,
		       COALESCE(NULLIF(ws.exercise_template_id, ''), m.exercise_template_id) AS template_id,
		       COALESCE(m.note, '') AS map_note
		FROM workout_sets ws
		LEFT JOIN exercise_name_map m
		       ON m.source = ws.source
		      AND m.source_key = COALESCE(NULLIF(ws.source_exercise_id, ''), ws.exercise_name)
		WHERE ws.session_date >= $2 AND ws.session_date < $3
		  AND ws.user_id = $4
		  AND NOT ws.is_warmup
	)`

// GetTrainingVolume returns sets per muscle group per period, split into the
// primary count and the count weighted by assisting muscles.
func (db *DB) GetTrainingVolume(ctx context.Context, start, end time.Time, bucket string, userID int) ([]TrainingVolumePeriod, error) {
	// Each set contributes one row for its primary muscle and one per assisting
	// muscle, so a single GROUP BY covers both counts.
	query := `WITH` + resolvedSetsCTE + `,
	expanded AS (
		SELECT r.session_date, r.weight_kg, r.reps, r.map_note,
		       t.primary_muscle_group AS muscle, 1::numeric AS weight, TRUE AS is_primary
		FROM resolved r JOIN exercise_templates t ON t.id = r.template_id
		UNION ALL
		SELECT r.session_date, r.weight_kg, r.reps, r.map_note,
		       s AS muscle, ` + fmt.Sprintf("%v", secondaryMuscleWeight) + `::numeric, FALSE
		FROM resolved r JOIN exercise_templates t ON t.id = r.template_id,
		     LATERAL unnest(t.secondary_muscle_groups) AS s
	)
	SELECT date_trunc($1, session_date)::date AS period,
	       muscle,
	       COUNT(*) FILTER (WHERE is_primary)::int,
	       SUM(weight)::float8,
	       COUNT(DISTINCT session_date::date)::int,
	       COALESCE(SUM(weight_kg * reps) FILTER (WHERE is_primary), 0)::float8,
	       COALESCE(SUM(weight) FILTER (WHERE map_note = 'approximate'), 0)::float8
	FROM expanded
	WHERE muscle <> ''
	GROUP BY period, muscle
	ORDER BY period DESC, SUM(weight) DESC`

	rows, err := db.Pool.Query(ctx, query, truncInterval(bucket), start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying training volume: %w", err)
	}
	defer rows.Close()

	periodMap := make(map[string]*TrainingVolumePeriod)
	var periodOrder []string

	for rows.Next() {
		var periodTime time.Time
		var mv MuscleVolume
		var approxSets float64
		if err := rows.Scan(&periodTime, &mv.Muscle, &mv.PrimarySets, &mv.WeightedSets,
			&mv.Sessions, &mv.TonnageKg, &approxSets); err != nil {
			return nil, fmt.Errorf("scanning muscle volume: %w", err)
		}
		if mv.WeightedSets > 0 {
			mv.ApproximatePct = math.Round(approxSets/mv.WeightedSets*1000) / 10
		}
		key := periodTime.Format("2006-01-02")
		if _, ok := periodMap[key]; !ok {
			periodMap[key] = &TrainingVolumePeriod{Period: key}
			periodOrder = append(periodOrder, key)
		}
		periodMap[key].Muscles = append(periodMap[key].Muscles, mv)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := db.addUnmappedSets(ctx, start, end, bucket, userID, periodMap, &periodOrder); err != nil {
		return nil, err
	}

	result := make([]TrainingVolumePeriod, 0, len(periodOrder))
	for _, key := range periodOrder {
		result = append(result, *periodMap[key])
	}
	return result, nil
}

// addUnmappedSets records the sets that reach no catalog entry, per period.
func (db *DB) addUnmappedSets(ctx context.Context, start, end time.Time, bucket string, userID int,
	periodMap map[string]*TrainingVolumePeriod, periodOrder *[]string) error {

	query := `WITH` + resolvedSetsCTE + `
	SELECT date_trunc($1, session_date)::date AS period,
	       COUNT(*)::int,
	       array_agg(DISTINCT exercise_name)
	FROM resolved
	WHERE template_id IS NULL
	GROUP BY period
	ORDER BY period DESC`

	rows, err := db.Pool.Query(ctx, query, truncInterval(bucket), start, end, userID)
	if err != nil {
		return fmt.Errorf("querying unmapped sets: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var periodTime time.Time
		var count int
		var names []string
		if err := rows.Scan(&periodTime, &count, &names); err != nil {
			return fmt.Errorf("scanning unmapped sets: %w", err)
		}
		key := periodTime.Format("2006-01-02")
		if _, ok := periodMap[key]; !ok {
			periodMap[key] = &TrainingVolumePeriod{Period: key}
			*periodOrder = append(*periodOrder, key)
		}
		periodMap[key].UnmappedSets = count
		periodMap[key].UnmappedNames = names
	}
	return rows.Err()
}

// ExerciseE1RM is one session's best estimated one-rep max for an exercise.
type ExerciseE1RM struct {
	Name      string  `json:"name"`
	Date      string  `json:"date"`
	E1RMKg    float64 `json:"e1rm_kg"`
	BestSetKg float64 `json:"best_set_kg"`
	Reps      int     `json:"reps"`
	// EffortRIR is reps in reserve, from whichever scale the source recorded.
	EffortRIR float64 `json:"effort_rir"`
	// RPE is the same value on the scale Hevy uses, for reading convenience.
	RPE float64 `json:"rpe"`
}

// epleyE1RM estimates a one-rep max from a working set.
//
// Epley's formula reads kg × (1 + reps/30). The reps in reserve are added to the
// repetition count because a set stopped two short of failure demonstrates the
// strength of a longer set: 8 reps at RIR 2 is treated as 10. Leaving that out
// understates every set not taken to failure, which in this data set is roughly
// half of them.
//
// The estimate degrades above about ten effective repetitions, where the linear
// term stops matching how strength actually declines.
func epleyE1RM(weightKg float64, reps int, effortRIR float64) float64 {
	if weightKg <= 0 || reps <= 0 {
		return 0
	}
	effective := float64(reps) + effortRIR
	return weightKg * (1 + effective/30)
}

// GetExerciseE1RM returns the best estimated one-rep max per session for each
// exercise, oldest first.
//
// Sets without an effort rating are excluded rather than assumed to be taken to
// failure — counting them as RIR 0 would understate them and add noise to a
// trend line.
func (db *DB) GetExerciseE1RM(ctx context.Context, start, end time.Time, userID int, exerciseFilter string) ([]ExerciseE1RM, error) {
	query := `SELECT session_date::date, exercise_name, weight_kg, reps, effort_rir
		FROM workout_sets
		WHERE session_date >= $1 AND session_date < $2
		  AND user_id = $3
		  AND NOT is_warmup
		  AND effort_rir IS NOT NULL
		  AND weight_kg > 0 AND reps > 0`
	args := []any{start, end, userID}
	if exerciseFilter != "" {
		query += ` AND exercise_name ILIKE '%' || $4 || '%'`
		args = append(args, exerciseFilter)
	}
	query += ` ORDER BY session_date ASC`

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying exercise e1rm: %w", err)
	}
	defer rows.Close()

	// Best estimate per exercise and day, kept in first-seen order.
	type key struct {
		name string
		date string
	}
	best := make(map[key]ExerciseE1RM)
	var order []key

	for rows.Next() {
		var d time.Time
		var name string
		var weight, effortRIR float64
		var reps int
		if err := rows.Scan(&d, &name, &weight, &reps, &effortRIR); err != nil {
			return nil, fmt.Errorf("scanning e1rm row: %w", err)
		}
		est := epleyE1RM(weight, reps, effortRIR)
		k := key{name: name, date: d.Format("2006-01-02")}
		if cur, ok := best[k]; !ok {
			order = append(order, k)
			best[k] = ExerciseE1RM{Name: name, Date: k.date, E1RMKg: est,
				BestSetKg: weight, Reps: reps, EffortRIR: effortRIR, RPE: 10 - effortRIR}
		} else if est > cur.E1RMKg {
			best[k] = ExerciseE1RM{Name: name, Date: k.date, E1RMKg: est,
				BestSetKg: weight, Reps: reps, EffortRIR: effortRIR, RPE: 10 - effortRIR}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]ExerciseE1RM, 0, len(order))
	for _, k := range order {
		e := best[k]
		e.E1RMKg = math.Round(e.E1RMKg*10) / 10
		result = append(result, e)
	}
	return result, nil
}
