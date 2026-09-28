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

// TestHealthKitCopiesOfDirectlySyncedProviders runs the source policy against
// the database: the Oura copy of a workout, as the Oura app wrote it into
// HealthKit on 2026-09-27, and an Oura category sample are dropped while Oura
// is connected and stored under "Oura" while it is not. An Oura heart rate
// sample is stored under "Oura" either way; the client rank decides at read
// time. Watch data in the same payload is stored as Apple Health either way.
func TestHealthKitCopiesOfDirectlySyncedProviders(t *testing.T) {
	db := sleepTestDB(t)
	ctx := context.Background()
	clear := func() {
		for _, stmt := range []string{
			`DELETE FROM health_metrics WHERE user_id = $1`,
			`DELETE FROM workouts WHERE user_id = $1`,
			`DELETE FROM category_samples WHERE user_id = $1`,
			`DELETE FROM oura_tokens WHERE user_id = $1`,
		} {
			if _, err := db.Pool.Exec(ctx, stmt, sleepTestUser); err != nil {
				t.Fatalf("clearing: %v", err)
			}
		}
	}
	clear()
	t.Cleanup(clear)

	payload := `{"data":{
	  "metrics":[{"name":"heart_rate","units":"count/min","data":[
	    {"date":"2026-09-27 08:10:00 +0000","qty":131,"source_bundle":"com.apple.health.5C1F0A77","source_name":"Linus Watch Ultra 2"},
	    {"date":"2026-09-27 08:10:02 +0000","qty":133,"source_bundle":"com.ouraring.oura","source_name":"Oura"}]}],
	  "workouts":[
	    {"id":"6ea3accf-579b-4648-966e-5bf62ba5e71d","name":"Running","start":"2026-09-27 08:08:32 +0000","end":"2026-09-27 08:32:46 +0000","duration":1454,
	     "source_bundle":"com.apple.health.5C1F0A77","source_name":"Linus Watch Ultra 2"},
	    {"id":"4b3ab45b-8bf2-43d7-aa31-554cb51a4227","name":"Running","start":"2026-09-27 08:08:00 +0000","end":"2026-09-27 08:33:00 +0000","duration":1500,
	     "source_bundle":"com.ouraring.oura","source_name":"Oura"}],
	  "category_samples":[
	    {"id":"0b8f7c2e-4a1d-4e6b-9c3f-2d5e8a7b1c90","type":"HKCategoryTypeIdentifierMindfulSession","value":0,
	     "start_date":"2026-09-27 06:00:00 +0000","end_date":"2026-09-27 06:10:00 +0000","source":"Oura","source_bundle":"com.ouraring.oura"}]}}`

	p := NewProvider(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	app := WithClient(ctx, ClientIOSApp)

	count := func(query string) int {
		t.Helper()
		var n int
		if err := db.Pool.QueryRow(ctx, query, sleepTestUser).Scan(&n); err != nil {
			t.Fatalf("counting: %v", err)
		}
		return n
	}
	check := func(label string, wantOuraCopies int) {
		t.Helper()
		if got := count(`SELECT count(*) FROM health_metrics WHERE user_id = $1 AND source = '' AND source_bundle = 'com.apple.health.5C1F0A77'`); got != 1 {
			t.Errorf("%s: %d Apple heart rate rows, want 1", label, got)
		}
		if got := count(`SELECT count(*) FROM workouts WHERE user_id = $1 AND source = ''`); got != 1 {
			t.Errorf("%s: %d Apple workouts, want 1", label, got)
		}
		if got := count(`SELECT count(*) FROM health_metrics WHERE user_id = $1 AND source = 'Oura' AND client = 'freereps_ios'`); got != 1 {
			t.Errorf("%s: %d Oura heart rate rows from the app, want 1", label, got)
		}
		for _, q := range []string{
			`SELECT count(*) FROM workouts WHERE user_id = $1 AND source = 'Oura'`,
			`SELECT count(*) FROM category_samples WHERE user_id = $1 AND source = 'Oura'`,
		} {
			if got := count(q); got != wantOuraCopies {
				t.Errorf("%s: %q = %d, want %d", label, q, got, wantOuraCopies)
			}
		}
	}

	// Oura connected: its copies are dropped.
	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO oura_tokens (user_id, client_id, client_secret, access_token) VALUES ($1, 'id', 'secret', 'token')`,
		sleepTestUser); err != nil {
		t.Fatalf("connecting Oura: %v", err)
	}
	res, err := p.Ingest(app, metricPayload(t, payload), sleepTestUser)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if res.SourceCopiesDropped != 2 {
		t.Errorf("dropped %d copies, want 2", res.SourceCopiesDropped)
	}
	check("connected", 0)

	// Oura not connected: HealthKit is the only path, so every copy is kept.
	clear()
	if _, err := p.Ingest(app, metricPayload(t, payload), sleepTestUser); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	check("not connected", 1)
}

// TestAppHeartRateFillsTheProvidersWorkout covers the Yoga session of
// 2026-09-28: the Oura API listed the workout without heart rate, and the Oura
// app's HealthKit copy of the heart rate arrived through the iOS app hours
// before the API's. The ingest of that copy gives the workout its series.
func TestAppHeartRateFillsTheProvidersWorkout(t *testing.T) {
	db := sleepTestDB(t)
	ctx := context.Background()
	clear := func() {
		for _, stmt := range []string{
			`DELETE FROM health_metrics WHERE user_id = $1`,
			`DELETE FROM workouts WHERE user_id = $1`,
			`DELETE FROM oura_tokens WHERE user_id = $1`,
		} {
			if _, err := db.Pool.Exec(ctx, stmt, sleepTestUser); err != nil {
				t.Fatalf("clearing: %v", err)
			}
		}
	}
	clear()
	t.Cleanup(clear)

	workoutID := uuid.MustParse("9637bffc-f481-55db-ae9b-1aefa0a8705e")
	if _, err := db.InsertWorkout(ctx, models.WorkoutRow{
		ID: workoutID, UserID: sleepTestUser, Name: "Yoga", Source: "Oura",
		StartTime: time.Date(2026, 9, 28, 6, 29, 34, 0, time.UTC),
		EndTime:   time.Date(2026, 9, 28, 8, 10, 55, 0, time.UTC),
	}); err != nil {
		t.Fatalf("inserting the Oura workout: %v", err)
	}

	p := NewProvider(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	res, err := p.Ingest(WithClient(ctx, ClientIOSApp), metricPayload(t, `{"data":{"metrics":[
	  {"name":"heart_rate","units":"count/min","data":[
	    {"date":"2026-09-28 06:40:00 +0000","qty":80,"source_bundle":"com.ouraring.oura","source_name":"Oura"},
	    {"date":"2026-09-28 06:40:30 +0000","qty":90,"source_bundle":"com.ouraring.oura","source_name":"Oura"},
	    {"date":"2026-09-28 07:10:00 +0000","qty":100,"source_bundle":"com.ouraring.oura","source_name":"Oura"},
	    {"date":"2026-09-28 07:10:00 +0000","qty":150,"source_bundle":"com.apple.health.5C1F0A77","source_name":"Linus Watch Ultra 2"}]}]}}`), sleepTestUser)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if res.WorkoutHRPoints != 2 {
		t.Errorf("filled %d minutes, want 2", res.WorkoutHRPoints)
	}

	var avg float64
	if err := db.Pool.QueryRow(ctx,
		`SELECT avg_heart_rate FROM workouts WHERE id = $1`, workoutID).Scan(&avg); err != nil {
		t.Fatalf("reading the workout: %v", err)
	}
	// Minutes 06:40 (85) and 07:10 (100); the Watch sample does not enter an
	// Oura workout.
	if avg != 92.5 {
		t.Errorf("avg_heart_rate = %v, want 92.5", avg)
	}
}
