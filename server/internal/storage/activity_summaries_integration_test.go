//go:build integration

// A day's activity summary grows until the day ends, and the iOS app sends it on
// every sync. Under ON CONFLICT DO NOTHING the first delivery stayed: 2026-09-18
// held 6 kcal of active energy against 1,287 kcal summed from the hourly rows.
// These tests pin the upsert against the real primary key.
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

const summaryTestUser = 4248

func summary(day time.Time, kcal float64) models.ActivitySummaryRow {
	return models.ActivitySummaryRow{UserID: summaryTestUser, Date: day, ActiveEnergy: qty(kcal), ActiveEnergyGoal: qty(800)}
}

func storedKcal(t *testing.T, db *DB, day time.Time) float64 {
	t.Helper()
	var v float64
	if err := db.Pool.QueryRow(context.Background(),
		`SELECT active_energy FROM activity_summaries WHERE user_id = $1 AND date = $2`,
		summaryTestUser, day).Scan(&v); err != nil {
		t.Fatalf("reading summary: %v", err)
	}
	return v
}

func TestLaterSummaryReplacesTheFirstOfTheDay(t *testing.T) {
	db := aggTestDB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DELETE FROM activity_summaries WHERE user_id = $1`, summaryTestUser); err != nil {
		t.Fatalf("clearing rows: %v", err)
	}
	day := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)

	if n, err := db.InsertActivitySummaries(ctx, []models.ActivitySummaryRow{summary(day, 6)}); err != nil || n != 1 {
		t.Fatalf("first delivery: n=%d err=%v", n, err)
	}
	if n, err := db.InsertActivitySummaries(ctx, []models.ActivitySummaryRow{summary(day, 1287)}); err != nil || n != 1 {
		t.Fatalf("later delivery: n=%d err=%v", n, err)
	}
	if got := storedKcal(t, db, day); got != 1287 {
		t.Fatalf("stored %v kcal, want the later delivery 1287", got)
	}
	if n, err := db.InsertActivitySummaries(ctx, []models.ActivitySummaryRow{summary(day, 1287)}); err != nil || n != 0 {
		t.Fatalf("identical delivery must write nothing: n=%d err=%v", n, err)
	}
	if _, err := db.InsertActivitySummaries(ctx, []models.ActivitySummaryRow{summary(day, 1300), summary(day, 1310)}); err != nil {
		t.Fatalf("batch with a repeated day: %v", err)
	}
	if got := storedKcal(t, db, day); got != 1310 {
		t.Fatalf("stored %v kcal, want the last row of the batch 1310", got)
	}
}
