//go:build integration

package health

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/claude/freereps/internal/models"
	"github.com/google/uuid"
)

// TestRouteCarrierJoinsDirectlySyncedWorkout: a workout Oura wrote to Apple
// Health reaches the server twice, from Oura's sync under Oura's id and as the
// route carrier of the iOS app under HealthKit's id. It must stay one workout,
// with the route, whichever arrives first.
func TestRouteCarrierJoinsDirectlySyncedWorkout(t *testing.T) {
	db := sleepTestDB(t)
	ctx := context.Background()
	for _, stmt := range []string{
		`DELETE FROM workout_routes WHERE user_id = $1`,
		`DELETE FROM workouts WHERE user_id = $1`,
	} {
		if _, err := db.Pool.Exec(ctx, stmt, sleepTestUser); err != nil {
			t.Fatalf("clearing: %v", err)
		}
	}
	p := NewProvider(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	app := WithClient(ctx, ClientIOSApp)

	oura := func(start time.Time) uuid.UUID {
		t.Helper()
		id := uuid.New()
		kcal := 149.0
		if _, err := db.InsertWorkout(ctx, models.WorkoutRow{
			ID: id, UserID: sleepTestUser, Name: "Walking", Source: "Oura",
			StartTime: start.Add(707 * time.Millisecond), EndTime: start.Add(28*time.Minute + 131*time.Millisecond),
			DurationSec: 1678, ActiveEnergyBurned: &kcal,
		}); err != nil {
			t.Fatal(err)
		}
		return id
	}
	carrier := func(id string, start string, end string, point string) {
		t.Helper()
		if _, err := p.Ingest(app, metricPayload(t, `{"data":{"workouts":[{"id":"`+id+`","name":"Walking",
		  "start":"`+start+`","end":"`+end+`","duration":1676,
		  "route":[{"latitude":52.6,"longitude":13.2,"altitude":54,"timestamp":"`+point+`"}]}]}}`), sleepTestUser); err != nil {
			t.Fatalf("ingest: %v", err)
		}
	}
	state := func() (workouts int, routeOwners []uuid.UUID) {
		t.Helper()
		if err := db.Pool.QueryRow(ctx, `SELECT count(*) FROM workouts WHERE user_id = $1`, sleepTestUser).Scan(&workouts); err != nil {
			t.Fatal(err)
		}
		rows, err := db.Pool.Query(ctx, `SELECT DISTINCT workout_id FROM workout_routes WHERE user_id = $1`, sleepTestUser)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			routeOwners = append(routeOwners, id)
		}
		return workouts, routeOwners
	}

	// Oura's sync first, then the carrier.
	first := oura(time.Date(2026, 11, 20, 8, 50, 49, 0, time.UTC))
	carrier("B3351D42-75F7-4984-AC57-5E77B5983EAA", "2026-11-20 08:50:49 +0000", "2026-11-20 09:18:48 +0000", "2026-11-20 08:50:50 +0000")
	if n, owners := state(); n != 1 || len(owners) != 1 || owners[0] != first {
		t.Fatalf("carrier after the oura workout: %d workouts, route on %v, want 1 workout with the route on %s", n, owners, first)
	}

	// The carrier first: it is stored, and merged once Oura's sync has delivered.
	carrier("91212F23-ED3F-45D4-B08E-3699C485B5B7", "2026-11-22 09:30:00 +0000", "2026-11-22 09:58:00 +0000", "2026-11-22 09:30:05 +0000")
	if n, _ := state(); n != 2 {
		t.Fatalf("carrier without a twin: %d workouts, want 2", n)
	}
	second := oura(time.Date(2026, 11, 22, 9, 30, 0, 0, time.UTC))
	merged, err := db.MergeRouteCarriers(ctx, sleepTestUser)
	if err != nil {
		t.Fatal(err)
	}
	n, owners := state()
	if merged != 1 || n != 2 || len(owners) != 2 {
		t.Fatalf("after the merge: merged %d, %d workouts, route owners %v; want 1, 2 and the two oura workouts", merged, n, owners)
	}
	for _, o := range owners {
		if o != first && o != second {
			t.Errorf("a route stays on %s, which is neither oura workout", o)
		}
	}

	// A workout of another time keeps its own row.
	carrier("72173EC4-41A4-4BF3-AB95-5EB489B9D4FB", "2026-11-23 10:00:00 +0000", "2026-11-23 10:30:00 +0000", "2026-11-23 10:00:05 +0000")
	if merged, _ := db.MergeRouteCarriers(ctx, sleepTestUser); merged != 0 {
		t.Errorf("merged %d carriers that have no twin", merged)
	}
	if n, _ := state(); n != 3 {
		t.Errorf("%d workouts, want 3", n)
	}
}
