//go:build integration

// exercise_name_map is keyed by the source's exercise id where a source
// delivers one (openGym) and by the exercise name otherwise (Alpha
// Progression). Which row a set reaches is decided by the join expression in
// three queries, so the test runs them against the schema the migrations build.
//
// Run with:
//
//	FREEREPS_TEST_DSN=postgres://user:pass@host:port/db?sslmode=disable \
//	  go test -tags integration ./internal/storage/
package storage

import (
	"context"
	"testing"
	"time"

	"github.com/claude/freereps/internal/models"
)

const mapTestUser = 4343

func TestExerciseMapResolvesByIDAndByName(t *testing.T) {
	db := aggTestDB(t)
	ctx := context.Background()

	// The catalog and the map are not per user, so the test uses keys no real
	// row carries and removes them again.
	const tplChest, tplLats = "TESTCHEST", "TESTLATS"
	cleanup := func() {
		_, _ = db.Pool.Exec(context.Background(), `DELETE FROM workout_sets WHERE user_id = $1`, mapTestUser)
		_, _ = db.Pool.Exec(context.Background(),
			`DELETE FROM exercise_name_map WHERE exercise_template_id IN ($1, $2) OR source_key LIKE 'test-%'`, tplChest, tplLats)
		_, _ = db.Pool.Exec(context.Background(),
			`DELETE FROM exercise_templates WHERE id IN ($1, $2)`, tplChest, tplLats)
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := db.UpsertExerciseTemplates(ctx, []ExerciseTemplate{
		{ID: tplChest, Title: "Test Bench", ExerciseType: "weight_reps", PrimaryMuscleGroup: "chest", SecondaryMuscleGroups: []string{}},
		{ID: tplLats, Title: "Test Chin Up", ExerciseType: "bodyweight_weighted", PrimaryMuscleGroup: "lats", SecondaryMuscleGroups: []string{}},
	}); err != nil {
		t.Fatalf("upserting templates: %v", err)
	}
	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO exercise_name_map (source, source_key, exercise_template_id) VALUES
		   ('openGym', 'test-0025', $1),
		   ('openGym', 'Test Chin Up', $2),
		   ('Alpha Progression', 'test-Bench Press', $1)`, tplChest, tplLats); err != nil {
		t.Fatalf("inserting map rows: %v", err)
	}

	day := time.Date(2026, 11, 24, 17, 0, 0, 0, time.UTC)
	set := func(source, externalID, name, sourceID string, exNum int) models.WorkoutSetRow {
		return models.WorkoutSetRow{
			UserID: mapTestUser, Source: source, ExternalID: externalID,
			SessionName: "Test", SessionDate: day, ExerciseNumber: exNum,
			ExerciseName: name, SourceExerciseID: sourceID, SetType: "normal",
			SetNumber: 1, WeightKg: 100, Reps: 5,
		}
	}
	if _, err := db.InsertWorkoutSets(ctx, []models.WorkoutSetRow{
		// Reaches the map through its id; the name is not a key.
		set("openGym", "w_1", "Bench Press (Barbell)", "test-0025", 1),
		// The name matches an openGym map row, but the set carries an id that
		// has none: the id decides, so the set stays unmapped.
		set("openGym", "w_1", "Test Chin Up", "test-4102", 2),
		// Alpha carries no id and reaches the map through its name.
		set("Alpha Progression", "", "test-Bench Press", "", 1),
	}); err != nil {
		t.Fatalf("inserting sets: %v", err)
	}

	sets, err := db.QueryWorkoutSets(ctx, day.Add(-time.Hour), day.Add(time.Hour), mapTestUser, "")
	if err != nil {
		t.Fatalf("querying sets: %v", err)
	}
	muscle := map[string]string{}
	for _, s := range sets {
		muscle[s.Source+"/"+s.ExerciseName] = s.PrimaryMuscleGroup
	}
	want := map[string]string{
		"openGym/Bench Press (Barbell)":      "chest",
		"openGym/Test Chin Up":               "",
		"Alpha Progression/test-Bench Press": "chest",
	}
	for k, v := range want {
		if got, ok := muscle[k]; !ok || got != v {
			t.Errorf("%s: muscle = %q (present %v), want %q", k, got, ok, v)
		}
	}

	unmapped, err := db.ListUnmappedExerciseNames(ctx, mapTestUser)
	if err != nil {
		t.Fatalf("listing unmapped: %v", err)
	}
	if len(unmapped) != 1 || unmapped[0].Source != "openGym" || unmapped[0].SourceKey != "test-4102" ||
		unmapped[0].ExerciseName != "Test Chin Up" || unmapped[0].Sets != 1 {
		t.Errorf("unmapped = %+v, want one openGym row keyed test-4102", unmapped)
	}

	periods, err := db.GetTrainingVolume(ctx, day.Add(-24*time.Hour), day.Add(24*time.Hour), "week", mapTestUser)
	if err != nil {
		t.Fatalf("training volume: %v", err)
	}
	if len(periods) != 1 {
		t.Fatalf("periods = %d, want 1", len(periods))
	}
	p := periods[0]
	if p.UnmappedSets != 1 {
		t.Errorf("unmapped_sets = %d, want 1", p.UnmappedSets)
	}
	var chest int
	for _, m := range p.Muscles {
		if m.Muscle == "chest" {
			chest = m.PrimarySets
		}
	}
	if chest != 2 {
		t.Errorf("chest primary sets = %d, want 2 (one openGym by id, one Alpha by name)", chest)
	}
}
