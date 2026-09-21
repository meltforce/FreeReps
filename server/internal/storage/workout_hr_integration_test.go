//go:build integration

// FillWorkoutHeartRateFromMetrics is SQL over two tables, and what it has to get
// right is the aggregation: Oura records a 5-second burst while a workout is
// running and returns to sparse sampling afterwards, so an average over
// samples reports the sampling rate as much as the heart rate. Against the real
// session of 2026-09-21 the two readings are 98.32 and 96.16. A unit test over
// the query string would prove the shape and none of that.
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

const hrTestUser = 4243

func hrTestDB(t *testing.T) *DB {
	t.Helper()
	db := aggTestDB(t)
	ctx := context.Background()
	for _, stmt := range []string{
		`DELETE FROM workout_heart_rate WHERE user_id = $1`,
		`DELETE FROM workouts WHERE user_id = $1`,
		`DELETE FROM health_metrics WHERE user_id = $1`,
	} {
		if _, err := db.Pool.Exec(ctx, stmt, hrTestUser); err != nil {
			t.Fatalf("clearing rows: %v", err)
		}
	}
	return db
}

func hrSample(t time.Time, source string, bpm float64) models.HealthMetricRow {
	return models.HealthMetricRow{
		Time: t, UserID: hrTestUser, MetricName: "heart_rate",
		Source: source, Units: "count/min", Qty: qty(bpm),
	}
}

func hrWorkout(t *testing.T, db *DB, start, end time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := db.InsertWorkout(context.Background(), models.WorkoutRow{
		ID: id, UserID: hrTestUser, Name: "Yoga", Source: "Oura",
		StartTime: start, EndTime: end,
	}); err != nil {
		t.Fatalf("inserting workout: %v", err)
	}
	return id
}

// TestDerivedHeartRateAveragesMinutesNotSamples is the regression test for the
// reading a dense burst produces: twelve samples in one minute and one sample in
// each of two later minutes give 111.43 over the samples and 80 over the
// minutes. It also covers the two exclusions — a sample outside the interval and
// a sample of another source — because both would otherwise land in the series
// of a workout they do not belong to.
func TestDerivedHeartRateAveragesMinutesNotSamples(t *testing.T) {
	db := hrTestDB(t)
	ctx := context.Background()

	start := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)
	id := hrWorkout(t, db, start, end)

	var rows []models.HealthMetricRow
	for i := 0; i < 12; i++ { // the burst: 5-second cadence within one minute
		rows = append(rows, hrSample(start.Add(time.Duration(i*5)*time.Second), "Oura", 120))
	}
	rows = append(rows,
		hrSample(start.Add(10*time.Minute), "Oura", 60),
		hrSample(start.Add(20*time.Minute), "Oura", 60),
		hrSample(start.Add(35*time.Minute), "Oura", 200), // after the workout
		hrSample(start.Add(5*time.Minute), "", 200),      // another source
	)
	insert(t, db, rows)

	n, err := db.FillWorkoutHeartRateFromMetrics(ctx, hrTestUser, id, "Oura", start, end)
	if err != nil {
		t.Fatalf("deriving heart rate: %v", err)
	}
	if n != 3 {
		t.Fatalf("inserted %d minute rows, want 3", n)
	}

	detail, err := db.GetWorkout(ctx, id, hrTestUser)
	if err != nil {
		t.Fatalf("reading workout: %v", err)
	}
	if len(detail.HeartRateData) != 3 {
		t.Fatalf("workout carries %d heart rate rows, want 3", len(detail.HeartRateData))
	}
	if detail.AvgHeartRate == nil || !nearly(*detail.AvgHeartRate, 80) {
		t.Errorf("avg_heart_rate = %v, want 80 — an average over samples would be 111.43", detail.AvgHeartRate)
	}
	if detail.MinHeartRate == nil || !nearly(*detail.MinHeartRate, 60) {
		t.Errorf("min_heart_rate = %v, want 60", detail.MinHeartRate)
	}
	if detail.MaxHeartRate == nil || !nearly(*detail.MaxHeartRate, 120) {
		t.Errorf("max_heart_rate = %v, want 120 — 200 lies outside the interval and in another source", detail.MaxHeartRate)
	}
}

// TestDerivedHeartRateIsIdempotent covers the sync re-reading the same days: the
// second cycle over a workout must add no row and must leave the summary where
// the first put it.
func TestDerivedHeartRateIsIdempotent(t *testing.T) {
	db := hrTestDB(t)
	ctx := context.Background()

	start := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	end := start.Add(10 * time.Minute)
	id := hrWorkout(t, db, start, end)
	insert(t, db, []models.HealthMetricRow{
		hrSample(start, "Oura", 100),
		hrSample(start.Add(time.Minute), "Oura", 110),
	})

	if _, err := db.FillWorkoutHeartRateFromMetrics(ctx, hrTestUser, id, "Oura", start, end); err != nil {
		t.Fatalf("first run: %v", err)
	}
	n, err := db.FillWorkoutHeartRateFromMetrics(ctx, hrTestUser, id, "Oura", start, end)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if n != 0 {
		t.Errorf("second run inserted %d rows, want 0", n)
	}

	detail, err := db.GetWorkout(ctx, id, hrTestUser)
	if err != nil {
		t.Fatalf("reading workout: %v", err)
	}
	if detail.AvgHeartRate == nil || !nearly(*detail.AvgHeartRate, 105) {
		t.Errorf("avg_heart_rate = %v, want 105", detail.AvgHeartRate)
	}
}

// TestDerivedHeartRateLeavesAWorkoutWithoutSamplesAlone guards the summary: a
// workout whose interval carries no sample of that source must keep the empty
// summary rather than receive one computed from nothing.
func TestDerivedHeartRateLeavesAWorkoutWithoutSamplesAlone(t *testing.T) {
	db := hrTestDB(t)
	ctx := context.Background()

	start := time.Date(2026, 9, 21, 14, 0, 0, 0, time.UTC)
	end := start.Add(20 * time.Minute)
	id := hrWorkout(t, db, start, end)

	n, err := db.FillWorkoutHeartRateFromMetrics(ctx, hrTestUser, id, "Oura", start, end)
	if err != nil {
		t.Fatalf("deriving heart rate: %v", err)
	}
	if n != 0 {
		t.Errorf("inserted %d rows although no sample lies in the interval, want 0", n)
	}

	detail, err := db.GetWorkout(ctx, id, hrTestUser)
	if err != nil {
		t.Fatalf("reading workout: %v", err)
	}
	if detail.AvgHeartRate != nil {
		t.Errorf("avg_heart_rate = %v, want none", *detail.AvgHeartRate)
	}
}
