//go:build integration

// A client that aggregates into hourly buckets sends the current hour while it is
// still filling and sends it again later. Under ON CONFLICT DO NOTHING the first,
// partial value stayed: on 2026-09-25 the 12:00Z step bucket kept 230.94 after a
// later sync delivered the full hour, while Apple Health showed 413. These tests
// pin the upsert that replaced it, against the real unique index.
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

const upsertTestUser = 4246

func upsertTestDB(t *testing.T) *DB {
	t.Helper()
	db := aggTestDB(t)
	if _, err := db.Pool.Exec(context.Background(),
		`DELETE FROM health_metrics WHERE user_id = $1`, upsertTestUser); err != nil {
		t.Fatalf("clearing rows: %v", err)
	}
	return db
}

func stepBucket(at time.Time, steps float64) models.HealthMetricRow {
	return models.HealthMetricRow{
		Time: at, UserID: upsertTestUser, MetricName: "step_count",
		Source: "", Units: "count", Qty: qty(steps),
	}
}

func storedSteps(t *testing.T, db *DB, at time.Time) float64 {
	t.Helper()
	var v float64
	if err := db.Pool.QueryRow(context.Background(),
		`SELECT qty FROM health_metrics WHERE user_id = $1 AND metric_name = 'step_count' AND source = '' AND time = $2`,
		upsertTestUser, at).Scan(&v); err != nil {
		t.Fatalf("reading bucket: %v", err)
	}
	return v
}

func TestLaterBucketReplacesPartialOne(t *testing.T) {
	db := upsertTestDB(t)
	ctx := context.Background()
	hour := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)

	if n, err := db.InsertHealthMetrics(ctx, []models.HealthMetricRow{stepBucket(hour, 230.94)}); err != nil || n != 1 {
		t.Fatalf("first delivery: n=%d err=%v", n, err)
	}
	n, err := db.InsertHealthMetrics(ctx, []models.HealthMetricRow{stepBucket(hour, 413)})
	if err != nil {
		t.Fatalf("second delivery: %v", err)
	}
	if n != 1 {
		t.Fatalf("a changed bucket must count as written, got %d", n)
	}
	if got := storedSteps(t, db, hour); got != 413 {
		t.Fatalf("stored %v, want the later delivery 413", got)
	}

	n, err = db.InsertHealthMetrics(ctx, []models.HealthMetricRow{stepBucket(hour, 413)})
	if err != nil {
		t.Fatalf("identical delivery: %v", err)
	}
	if n != 0 {
		t.Fatalf("an identical re-delivery must write nothing, got %d", n)
	}
}

// One payload can carry the same key twice; ON CONFLICT DO UPDATE rejects a
// statement that touches one row twice, so the batch must be deduplicated first.
func TestRepeatedKeyInOneBatchKeepsTheLast(t *testing.T) {
	db := upsertTestDB(t)
	ctx := context.Background()
	hour := time.Date(2026, 9, 25, 13, 0, 0, 0, time.UTC)

	if _, err := db.InsertHealthMetrics(ctx, []models.HealthMetricRow{
		stepBucket(hour, 100), stepBucket(hour, 150),
	}); err != nil {
		t.Fatalf("batch with a repeated key: %v", err)
	}
	if got := storedSteps(t, db, hour); got != 150 {
		t.Fatalf("stored %v, want the last row of the batch 150", got)
	}
}
