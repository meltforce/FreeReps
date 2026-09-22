//go:build integration

// The deduplication in QueryWorkouts is a window function over a 5-minute
// bucket, and what the list needs from it besides the winning row is the set of
// sources that reported the session. Both are SQL; a unit test over the query
// string would not show which row survives or what it carries.
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
	"github.com/google/uuid"
)

func listWorkout(t *testing.T, db *DB, name, source string, start time.Time) {
	t.Helper()
	if _, err := db.InsertWorkout(context.Background(), models.WorkoutRow{
		ID: uuid.New(), UserID: hrTestUser, Name: name, Source: source,
		StartTime: start, EndTime: start.Add(30 * time.Minute),
	}); err != nil {
		t.Fatalf("inserting workout: %v", err)
	}
}

// TestWorkoutListReportsEverySourceOfTheWindow is the regression test for a
// session recorded in the Oura app being labelled "Apple Health": it arrives
// over the API and as the copy the Oura app writes into HealthKit, the priority
// ranks the copy first, and the copy carries no source name. The surviving row
// has to name both, or the list reports the hub as the recorder.
func TestWorkoutListReportsEverySourceOfTheWindow(t *testing.T) {
	db := hrTestDB(t)
	ctx := context.Background()

	if err := db.UpsertSourcePriority(ctx, hrTestUser, "activity", []string{"", "Oura"}); err != nil {
		t.Fatalf("setting priority: %v", err)
	}
	t.Cleanup(func() {
		_ = db.DeleteSourcePriority(context.Background(), hrTestUser, "activity")
	})

	day := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	both := day.Add(10 * time.Hour)
	listWorkout(t, db, "Yoga", "Oura", both)
	listWorkout(t, db, "Yoga", "", both.Add(21*time.Second)) // same 5-minute bucket
	listWorkout(t, db, "Walking", "", day.Add(12*time.Hour)) // Apple Health alone

	got, err := db.QueryWorkouts(ctx, day, day.Add(24*time.Hour), hrTestUser, "")
	if err != nil {
		t.Fatalf("querying workouts: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d workouts, want 2 — the pair has to collapse into one row", len(got))
	}

	var paired, alone *models.WorkoutRow
	for i := range got {
		switch got[i].Name {
		case "Yoga":
			paired = &got[i]
		case "Walking":
			alone = &got[i]
		}
	}
	if paired == nil || alone == nil {
		t.Fatalf("expected one Yoga and one Walking row, got %+v", got)
	}

	if paired.Source != "" {
		t.Errorf("surviving source = %q, want the empty one — Apple Health ranks first", paired.Source)
	}
	if len(paired.Sources) != 2 || paired.Sources[0] != "" || paired.Sources[1] != "Oura" {
		t.Errorf("Sources = %q, want [\"\" \"Oura\"] so the list can name the recorder", paired.Sources)
	}
	if paired.RecordedBy != "Oura" {
		t.Errorf("RecordedBy of the pair = %q, want \"Oura\" — the row the list shows is the HealthKit copy", paired.RecordedBy)
	}
	if alone.RecordedBy != "Apple Health" {
		t.Errorf("RecordedBy of the unpaired workout = %q, want \"Apple Health\"", alone.RecordedBy)
	}
	if len(alone.Sources) != 1 || alone.Sources[0] != "" {
		t.Errorf("Sources of the unpaired workout = %q, want [\"\"]", alone.Sources)
	}
}
