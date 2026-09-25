//go:build integration

// The iOS app and Health Auto Export both deliver Apple Health data with
// source = ''. The app writes one row per hour, Health Auto Export one per
// minute; before the client column both were summed, so 2026-09-19 counted
// 11,628 + 12,007 steps. And an hourly row and a minute row at the full hour
// shared one key and overwrote each other. These tests pin the client rank
// (freereps_ios before hae before unmarked rows) against the real tables.
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

const clientTestUser = 4247

func clientTestDB(t *testing.T) *DB {
	t.Helper()
	db := aggTestDB(t)
	if _, err := db.Pool.Exec(context.Background(),
		`DELETE FROM health_metrics WHERE user_id = $1`, clientTestUser); err != nil {
		t.Fatalf("clearing rows: %v", err)
	}
	return db
}

func appleSteps(at time.Time, client string, steps float64) models.HealthMetricRow {
	return models.HealthMetricRow{
		Time: at, UserID: clientTestUser, MetricName: "step_count",
		Source: "", Client: client, Units: "count", Qty: qty(steps),
	}
}

func dailySteps(t *testing.T, db *DB) float64 {
	t.Helper()
	sums, err := db.GetDailySums(context.Background(), clientTestUser, []string{"step_count"})
	if err != nil {
		t.Fatalf("GetDailySums: %v", err)
	}
	if len(sums) != 1 {
		t.Fatalf("expected one daily sum, got %v", sums)
	}
	return sums[0].Total
}

func TestAppRowsWinOverHealthAutoExportRows(t *testing.T) {
	db := clientTestDB(t)
	ctx := context.Background()
	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)

	var rows []models.HealthMetricRow
	// The app: two hourly sums. Health Auto Export: minute rows for the same hours,
	// one of them at the full hour, where it shares time with the app's row.
	rows = append(rows, appleSteps(day.Add(10*time.Hour), "freereps_ios", 600), appleSteps(day.Add(11*time.Hour), "freereps_ios", 400))
	rows = append(rows, appleSteps(day.Add(10*time.Hour), "hae", 20))
	for m := 1; m < 60; m++ {
		rows = append(rows, appleSteps(day.Add(10*time.Hour+time.Duration(m)*time.Minute), "hae", 10))
	}
	if _, err := db.InsertHealthMetrics(ctx, rows); err != nil {
		t.Fatalf("inserting: %v", err)
	}

	if got := dailySteps(t, db); got != 1000 {
		t.Fatalf("daily steps %v, want the app's 1000 alone (Health Auto Export rows must not add)", got)
	}

	var n int
	if err := db.Pool.QueryRow(ctx,
		`SELECT count(*) FROM health_metrics WHERE user_id = $1 AND time = $2`,
		clientTestUser, day.Add(10*time.Hour)).Scan(&n); err != nil {
		t.Fatalf("counting: %v", err)
	}
	if n != 2 {
		t.Fatalf("the app's hourly row and Health Auto Export's minute row at 10:00 must both be stored, got %d rows", n)
	}
}

// Where the app delivered nothing for a day, Health Auto Export's rows count, and
// they win over unmarked rows stored before the client column existed.
func TestHealthAutoExportRowsCountWithoutAppRows(t *testing.T) {
	db := clientTestDB(t)
	ctx := context.Background()
	day := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)

	rows := []models.HealthMetricRow{
		appleSteps(day.Add(8*time.Hour), "hae", 300),
		appleSteps(day.Add(9*time.Hour), "hae", 200),
		appleSteps(day.Add(8*time.Hour+30*time.Minute), "", 999),
	}
	if _, err := db.InsertHealthMetrics(ctx, rows); err != nil {
		t.Fatalf("inserting: %v", err)
	}
	if got := dailySteps(t, db); got != 500 {
		t.Fatalf("daily steps %v, want Health Auto Export's 500", got)
	}
}
