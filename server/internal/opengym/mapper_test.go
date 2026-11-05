package opengym

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"testing"
	"time"
)

// testdata/state-cut.json is cut from the operator's state of 2026-11-05 (rev 3):
// the chin-up entry of workout iw2do12qenuxn and the bench press and pull-up
// entries of iwfp6ie5n94d, every other key of the state removed. The cases the
// captured state does not contain — a workout without start, a set that was not
// done, a custom exercise, pounds, an assisted exercise, unobserved set shapes —
// are built on top of it in the tests below.

var testCatalog = map[string]Exercise{
	"0025": {TemplateID: "79D0BB3A", Title: "Bench Press (Barbell)", ExerciseType: "weight_reps"},
	"0572": {TemplateID: "D23C609B", Title: "Chin Up (Assisted)", ExerciseType: "bodyweight_assisted"},
	"0652": {TemplateID: "729237D1", Title: "Pull Up (Weighted)", ExerciseType: "bodyweight_weighted"},
	"1326": {TemplateID: "023943F1", Title: "Chin Up (Weighted)", ExerciseType: "bodyweight_weighted"},
}

func loadState(t *testing.T) *State {
	t.Helper()
	raw, err := os.ReadFile("testdata/state-cut.json")
	if err != nil {
		t.Fatal(err)
	}
	var resp DataResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("decoding fixture: %v", err)
	}
	return &resp.State
}

func berlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func testMapper(t *testing.T) Mapper {
	return Mapper{UserID: 7, Location: berlin(t), Catalog: testCatalog}
}

func workoutByID(t *testing.T, st *State, id string) Workout {
	t.Helper()
	for _, w := range st.Workouts {
		if w.ID == id {
			return w
		}
	}
	t.Fatalf("workout %s not in fixture", id)
	return Workout{}
}

func TestDecodeFixture(t *testing.T) {
	st := loadState(t)
	if st.Rev != 3 || st.WID != "cec6ae102e8461ff" {
		t.Errorf("(rev, wid) = (%d, %q), want (3, cec6ae102e8461ff)", st.Rev, st.WID)
	}
	if st.Unit != "kg" {
		t.Errorf("unit = %q, want kg", st.Unit)
	}
	if len(st.Workouts) != 2 {
		t.Fatalf("workouts = %d, want 2", len(st.Workouts))
	}
}

// The upper session of 2026-11-05 as it was logged: three warm-ups and three
// working sets of bench press, one warm-up and three weighted pull-ups.
func TestMapWorkoutRealSession(t *testing.T) {
	st := loadState(t)
	w := workoutByID(t, st, "iwfp6ie5n94d")
	rows, err := testMapper(t).MapWorkout(st, w)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 10 {
		t.Fatalf("rows = %d, want 10", len(rows))
	}

	wantStart := time.UnixMilli(1793857222000)
	wantEnd := time.UnixMilli(1793863673000)
	for i, r := range rows {
		if r.Source != SourceName || r.ExternalID != "iwfp6ie5n94d" || r.UserID != 7 {
			t.Errorf("row %d: source/external_id/user = %q/%q/%d", i, r.Source, r.ExternalID, r.UserID)
		}
		if !r.SessionDate.Equal(wantStart) {
			t.Errorf("row %d: session_date = %v, want %v", i, r.SessionDate, wantStart)
		}
		if r.SessionEnd == nil || !r.SessionEnd.Equal(wantEnd) {
			t.Errorf("row %d: session_end = %v, want %v", i, r.SessionEnd, wantEnd)
		}
		if r.SessionDuration != "1:47 hr" {
			t.Errorf("row %d: duration = %q, want 1:47 hr", i, r.SessionDuration)
		}
		if r.RoutineID != "" {
			t.Errorf("row %d: routine_id = %q, want empty for routineId null", i, r.RoutineID)
		}
		if r.SessionName != "Upper · Block 2026-10" {
			t.Errorf("row %d: session_name = %q", i, r.SessionName)
		}
		// A non-empty template id would be read as a Hevy template id.
		if r.ExerciseTemplateID != "" {
			t.Errorf("row %d: exercise_template_id = %q, want empty", i, r.ExerciseTemplateID)
		}
		if r.RIR != nil {
			t.Errorf("row %d: RIR = %v, want nil — the state carries RPE", i, *r.RIR)
		}
	}

	tests := []struct {
		row            int
		exerciseNumber int
		sourceID, name string
		setNumber      int
		warmup         bool
		setType        string
		weight         float64
		reps           int
		rpe            float64
		bodyweightPlus bool
	}{
		{0, 1, "0025", "Bench Press (Barbell)", 1, true, "warmup", 20, 10, 6, false},
		{3, 1, "0025", "Bench Press (Barbell)", 4, false, "normal", 102.5, 6, 8.5, false},
		{5, 1, "0025", "Bench Press (Barbell)", 6, false, "normal", 102.5, 6, 9.5, false},
		{6, 2, "0652", "Pull Up (Weighted)", 1, true, "warmup", 0, 6, 7, true},
		{7, 2, "0652", "Pull Up (Weighted)", 2, false, "normal", 5, 8, 8, true},
	}
	for _, tt := range tests {
		r := rows[tt.row]
		if r.ExerciseNumber != tt.exerciseNumber || r.SourceExerciseID != tt.sourceID || r.ExerciseName != tt.name {
			t.Errorf("row %d: exercise = %d/%q/%q, want %d/%q/%q", tt.row,
				r.ExerciseNumber, r.SourceExerciseID, r.ExerciseName, tt.exerciseNumber, tt.sourceID, tt.name)
		}
		if r.SetNumber != tt.setNumber || r.IsWarmup != tt.warmup || r.SetType != tt.setType {
			t.Errorf("row %d: set = %d/%v/%q, want %d/%v/%q", tt.row,
				r.SetNumber, r.IsWarmup, r.SetType, tt.setNumber, tt.warmup, tt.setType)
		}
		if r.WeightKg != tt.weight || r.Reps != tt.reps {
			t.Errorf("row %d: %v kg × %d, want %v kg × %d", tt.row, r.WeightKg, r.Reps, tt.weight, tt.reps)
		}
		if r.RPE == nil || *r.RPE != tt.rpe {
			t.Errorf("row %d: RPE = %v, want %v", tt.row, r.RPE, tt.rpe)
		}
		if r.IsBodyweightPlus != tt.bodyweightPlus {
			t.Errorf("row %d: is_bodyweight_plus = %v, want %v", tt.row, r.IsBodyweightPlus, tt.bodyweightPlus)
		}
	}
}

// The weighted chin-ups were imported under 0572 (lever assisted chin-up) and
// moved to 1326 on 2026-11-05. Under 1326 the weight is added load.
func TestMapWorkoutWeightedChinUp(t *testing.T) {
	st := loadState(t)
	rows, err := testMapper(t).MapWorkout(st, workoutByID(t, st, "iw2do12qenuxn"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(rows))
	}
	top := rows[3]
	if top.ExerciseName != "Chin Up (Weighted)" || top.WeightKg != 10 || top.Reps != 9 || !top.IsBodyweightPlus {
		t.Errorf("top set = %q %v kg × %d bodyweight_plus=%v, want Chin Up (Weighted) 10 kg × 9 true",
			top.ExerciseName, top.WeightKg, top.Reps, top.IsBodyweightPlus)
	}
}

func intp(i int) *int           { return &i }
func floatp(f float64) *float64 { return &f }
func int64p(i int64) *int64     { return &i }

func TestMapWorkoutWithoutStartUsesDayInSessionTimezone(t *testing.T) {
	st := loadState(t)
	w := workoutByID(t, st, "iwfp6ie5n94d")
	w.Start, w.End = nil, nil

	rows, err := testMapper(t).MapWorkout(st, w)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 11, 5, 0, 0, 0, 0, berlin(t))
	if !rows[0].SessionDate.Equal(want) {
		t.Errorf("session_date = %v, want %v", rows[0].SessionDate, want)
	}
	if rows[0].SessionEnd != nil || rows[0].SessionDuration != "" {
		t.Errorf("session_end/duration = %v/%q, want nil/empty", rows[0].SessionEnd, rows[0].SessionDuration)
	}

	m := testMapper(t)
	m.Location = nil
	if _, err := m.MapWorkout(st, w); err == nil {
		t.Error("no start and no timezone mapped without error")
	}
}

// A planned set that was not performed is not stored, and the numbering of the
// performed sets has no gap where it was.
func TestMapWorkoutSkipsSetsNotDone(t *testing.T) {
	st := loadState(t)
	w := workoutByID(t, st, "iw2do12qenuxn")
	sets := w.Entries[0].Sets
	notDone := Set{Weight: floatp(10), Reps: intp(8), Done: false}
	w.Entries[0].Sets = []Set{sets[0], sets[1], notDone, sets[2]}

	rows, err := testMapper(t).MapWorkout(st, w)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	for i, r := range rows {
		if r.SetNumber != i+1 {
			t.Errorf("row %d: set_number = %d, want %d", i, r.SetNumber, i+1)
		}
	}
}

func TestMapWorkoutExerciseNames(t *testing.T) {
	st := loadState(t)
	st.CustomEx = []CustomExercise{{ID: "cx1", Name: "Landmine row", Equipment: "barbell"}}
	w := Workout{ID: "w_1", Day: "2026-11-24", Start: int64p(1795500000000), Entries: []Entry{
		{ID: "cx1", Sets: []Set{{Weight: floatp(40), Reps: intp(10), Done: true}}},
		{ID: "4102", Sets: []Set{{Weight: floatp(5), Reps: intp(8), Done: true}}},
	}}

	rows, err := testMapper(t).MapWorkout(st, w)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].ExerciseName != "Landmine row" || rows[0].SourceExerciseID != "cx1" {
		t.Errorf("custom exercise = %q/%q, want Landmine row/cx1", rows[0].ExerciseName, rows[0].SourceExerciseID)
	}
	// An unmapped catalogue id is stored, named after its id, and appears in
	// unmapped_sets until a map row is added.
	if rows[1].ExerciseName != "openGym exercise 4102" || rows[1].SourceExerciseID != "4102" {
		t.Errorf("unmapped exercise = %q/%q, want openGym exercise 4102/4102", rows[1].ExerciseName, rows[1].SourceExerciseID)
	}
	if rows[1].WeightKg != 5 {
		t.Errorf("unmapped exercise weight = %v, want 5", rows[1].WeightKg)
	}
}

func TestMapWorkoutConvertsPounds(t *testing.T) {
	st := loadState(t)
	st.Unit = "lb"
	rows, err := testMapper(t).MapWorkout(st, workoutByID(t, st, "iw2do12qenuxn"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := rows[1].WeightKg, 10*0.45359237; math.Abs(got-want) > 1e-9 {
		t.Errorf("10 lb = %v kg, want %v", got, want)
	}
}

func TestMapWorkoutRefusesUnsupported(t *testing.T) {
	base := func() (*State, Workout) {
		st := loadState(t)
		return st, workoutByID(t, st, "iw2do12qenuxn")
	}
	tests := []struct {
		name   string
		modify func(w *Workout)
	}{
		{"assisted exercise", func(w *Workout) { w.Entries[0].ID = "0572" }},
		{"rest-pause set", func(w *Workout) { w.Entries[0].Sets[1].Type = "restpause" }},
		{"per-side set", func(w *Workout) { w.Entries[0].Sets[1].Sides = json.RawMessage(`{"left":5,"right":5}`) }},
		{"timed set", func(w *Workout) { w.Entries[0].Sets[1].Sec = json.RawMessage(`45`) }},
		{"cardio set", func(w *Workout) { w.Entries[0].Sets[1].Speed = json.RawMessage(`10.5`) }},
		{"set without reps", func(w *Workout) { w.Entries[0].Sets[1].Reps = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, w := base()
			w.Entries[0].Sets = append([]Set(nil), w.Entries[0].Sets...)
			tt.modify(&w)
			_, err := testMapper(t).MapWorkout(st, w)
			if !errors.Is(err, ErrUnsupported) {
				t.Errorf("err = %v, want ErrUnsupported", err)
			}
		})
	}

	// An unobserved field on a set that was not done does not block the workout:
	// the set is not stored either way.
	st, w := base()
	w.Entries[0].Sets = append([]Set(nil), w.Entries[0].Sets...)
	w.Entries[0].Sets[1].Done = false
	w.Entries[0].Sets[1].Sec = json.RawMessage(`45`)
	if _, err := testMapper(t).MapWorkout(st, w); err != nil {
		t.Errorf("not-done set with an unobserved field: err = %v, want nil", err)
	}
}

func TestDecodeNullFieldsAsAbsent(t *testing.T) {
	var s Set
	if err := json.Unmarshal([]byte(`{"w":5,"r":8,"done":true,"sec":null}`), &s); err != nil {
		t.Fatal(err)
	}
	if shape := unobservedShape(s); shape != "" {
		t.Errorf("sec:null reported as %q, want a plain set", shape)
	}
}
