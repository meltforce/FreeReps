//go:build integration

package storage

import (
	"context"
	"testing"
	"time"
)

// TestRunHistoryCountsTheSourcesOwnColumns catches the stale-source rule reading
// the wrong column: Hevy writes workouts and sets and never metrics_inserted, so
// a predicate on metrics_inserted alone would report a Hevy user who trains
// every day as never having stored a row. It also checks that the newest status
// is the newest run's, since the rule stays silent when that run failed.
func TestRunHistoryCountsTheSourcesOwnColumns(t *testing.T) {
	db := aggTestDB(t)
	ctx := context.Background()
	runHistoryTestUser, err := db.GetOrCreateUser(ctx, "run-history-test", "")
	if err != nil {
		t.Fatalf("creating user: %v", err)
	}
	if _, err := db.Pool.Exec(ctx, `DELETE FROM import_logs WHERE user_id = $1`, runHistoryTestUser); err != nil {
		t.Fatalf("clearing rows: %v", err)
	}

	stored := time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)
	newest := time.Date(2026, 9, 27, 8, 0, 0, 0, time.UTC)
	for _, r := range []struct {
		at                    time.Time
		status                string
		workouts, sets, metrs int64
	}{
		{stored, "success", 1, 12, 0},
		{stored.Add(24 * time.Hour), "success", 0, 0, 0},
		{newest, "error", 0, 0, 0},
	} {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO import_logs (user_id, created_at, source, status, workouts_inserted, sets_inserted, metrics_inserted)
			 VALUES ($1, $2, 'hevy_sync', $3, $4, $5, $6)`,
			runHistoryTestUser, r.at, r.status, r.workouts, r.sets, r.metrs); err != nil {
			t.Fatalf("inserting import log: %v", err)
		}
	}

	byUser, err := db.RunHistoryByUser(ctx, "hevy_sync")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := byUser[runHistoryTestUser]
	if !ok {
		t.Fatal("no history for the test user")
	}
	if !got.LastStoredAt.Equal(stored) {
		t.Errorf("LastStoredAt = %s, want %s", got.LastStoredAt, stored)
	}
	if !got.NewestRunAt.Equal(newest) || got.NewestStatus != "error" {
		t.Errorf("newest = %s %q, want %s \"error\"", got.NewestRunAt, got.NewestStatus, newest)
	}
}
