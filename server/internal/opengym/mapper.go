package opengym

import (
	"errors"
	"fmt"
	"time"

	"github.com/claude/freereps/internal/models"
)

// SourceName identifies rows in workout_sets that came from openGym. It is part
// of the table's natural key and the source column of exercise_name_map.
const SourceName = "openGym"

// kgPerLb is the factor openGym itself converts with (frontend/src/lib/units.js
// LB_KG), so a pound value round-trips to the kilogram value the app shows.
const kgPerLb = 0.45359237

// ErrUnsupported marks a workout the mapper refuses rather than storing it with
// a meaning it cannot vouch for: an assisted exercise, or a set shape that has
// not been observed in a captured state. The caller skips the workout and
// reports it; the rest of the state is unaffected.
var ErrUnsupported = errors.New("unsupported openGym workout")

// Exercise is the catalog entry an openGym exercise id is mapped onto, read
// from exercise_name_map joined with exercise_templates.
type Exercise struct {
	TemplateID   string
	Title        string
	ExerciseType string // Hevy's exercise_type, e.g. bodyweight_weighted
}

// Mapper converts openGym workouts into workout_sets rows.
type Mapper struct {
	UserID int
	// Location is the configured session timezone (ingest.session_timezone).
	// It places a workout that carries only its day, as for Alpha
	// (DECISIONS.md, 2026-08-10).
	Location *time.Location
	// Catalog holds the mapped exercises, keyed by openGym exercise id.
	Catalog map[string]Exercise
}

// MapWorkout converts one workout of st into workout_sets rows.
//
// The exercise name is the title of the mapped catalog entry, so a lift keeps
// the name it had under Hevy and per-exercise series and name filters run
// across the source change. A custom exercise keeps its openGym name; an
// unmapped catalogue id is named after the id and counts as unmapped until a
// map row is added.
//
// exercise_template_id stays empty: the read path treats any non-empty value
// as a Hevy template id. The openGym id goes into SourceExerciseID, which is
// what exercise_name_map is keyed by for this source.
func (m Mapper) MapWorkout(st *State, w Workout) ([]models.WorkoutSetRow, error) {
	start, err := m.sessionStart(w)
	if err != nil {
		return nil, err
	}

	var endPtr *time.Time
	var duration string
	if w.End != nil {
		end := time.UnixMilli(*w.End)
		if end.After(start) {
			endPtr = &end
			duration = formatDuration(end.Sub(start))
		}
	}

	routineID := ""
	if w.RoutineID != nil {
		routineID = *w.RoutineID
	}
	name := w.Name
	if name == "" {
		name = "Workout"
	}

	factor := 1.0
	if st.Unit == "lb" {
		factor = kgPerLb
	}

	var rows []models.WorkoutSetRow
	for i, entry := range w.Entries {
		exName, bodyweightPlus, err := m.resolve(st, entry.ID)
		if err != nil {
			return nil, fmt.Errorf("workout %s, exercise %s: %w", w.ID, entry.ID, err)
		}

		setNumber := 0
		for j, set := range entry.Sets {
			if !set.Done {
				continue
			}
			if shape := unobservedShape(set); shape != "" {
				return nil, fmt.Errorf("workout %s, exercise %s, set %d: %s set: %w",
					w.ID, entry.ID, j+1, shape, ErrUnsupported)
			}
			if set.Reps == nil {
				return nil, fmt.Errorf("workout %s, exercise %s, set %d: no reps: %w",
					w.ID, entry.ID, j+1, ErrUnsupported)
			}
			setNumber++

			row := models.WorkoutSetRow{
				UserID:           m.UserID,
				Source:           SourceName,
				ExternalID:       w.ID,
				RoutineID:        routineID,
				SessionName:      name,
				SessionDate:      start,
				SessionEnd:       endPtr,
				SessionDuration:  duration,
				ExerciseNumber:   i + 1,
				ExerciseName:     exName,
				SourceExerciseID: entry.ID,
				IsWarmup:         set.Phase == "warmup",
				SetType:          setType(set),
				SetNumber:        setNumber,
				IsBodyweightPlus: bodyweightPlus,
				Reps:             *set.Reps,
				RPE:              set.RPE,
				RIR:              set.RIR,
			}
			if set.Weight != nil {
				row.WeightKg = *set.Weight * factor
			}
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// sessionStart is the workout's start instant, or its day at midnight in the
// configured timezone when the app recorded no start.
func (m Mapper) sessionStart(w Workout) (time.Time, error) {
	if w.Start != nil {
		return time.UnixMilli(*w.Start), nil
	}
	loc := m.Location
	if loc == nil {
		return time.Time{}, fmt.Errorf("workout %s has no start and no session timezone is configured", w.ID)
	}
	day, err := time.ParseInLocation("2006-01-02", w.Day, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("workout %s: parsing day %q: %w", w.ID, w.Day, err)
	}
	return day, nil
}

// resolve returns the stored name of an exercise id and whether its weight is
// load added to body weight. It refuses an assisted exercise, whose weight is
// assistance and would count as load in tonnage.
func (m Mapper) resolve(st *State, id string) (name string, bodyweightPlus bool, err error) {
	if ex, ok := m.Catalog[id]; ok {
		if ex.ExerciseType == "bodyweight_assisted" {
			return "", false, fmt.Errorf("assisted exercise %q, whose weight is assistance rather than load: %w",
				ex.Title, ErrUnsupported)
		}
		return ex.Title, ex.ExerciseType == "bodyweight_weighted", nil
	}
	for _, c := range st.CustomEx {
		if c.ID == id {
			n := c.Name
			if n == "" {
				n = "openGym exercise " + id
			}
			return n, c.Equipment == "body weight" || c.Equipment == "weighted", nil
		}
	}
	return "openGym exercise " + id, false, nil
}

// setType maps a set onto the workout_sets vocabulary Hevy rows use.
func setType(s Set) string {
	switch {
	case s.Phase == "warmup":
		return "warmup"
	case s.Failure:
		return "failure"
	default:
		return "normal"
	}
}

// unobservedShape names the first field of s that belongs to a set shape not
// yet seen in a captured state, or returns "" for a plain weight-and-reps set.
func unobservedShape(s Set) string {
	switch {
	case s.Type != "":
		return "type " + s.Type
	case present(s.Sides):
		return "per-side"
	case present(s.Clusters):
		return "cluster"
	case present(s.Sec), present(s.Min):
		return "timed"
	case present(s.Speed), present(s.Incline):
		return "cardio"
	}
	return ""
}

func present(raw []byte) bool {
	return len(raw) > 0 && string(raw) != "null"
}

// formatDuration renders a session length in the "H:MM hr" form the Alpha and
// Hevy ingests write, so every source displays identically.
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%02d hr", int(d.Hours()), int(d.Minutes())%60)
}
