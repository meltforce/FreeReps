//go:build integration

// The aggregation rules in this package are SQL, so a unit test that inspects
// the generated string proves the shape and nothing about the figures. These
// tests run the queries against a real TimescaleDB with the row layout that
// produced the wrong numbers in production on 2026-09-18.
//
// Run with:
//
//	FREEREPS_TEST_DSN=postgres://user:pass@host:port/db?sslmode=disable \
//	  go test -tags integration ./internal/storage/
package storage

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/claude/freereps/internal/models"
)

const aggTestUser = 4242

func aggTestDB(t *testing.T) *DB {
	t.Helper()
	dsn := os.Getenv("FREEREPS_TEST_DSN")
	if dsn == "" {
		t.Skip("FREEREPS_TEST_DSN not set")
	}

	migrations, err := filepath.Abs("../../migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	if err := RunMigrations(dsn, migrations); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	ctx := context.Background()
	db, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(db.Close)

	var dbName string
	if err := db.Pool.QueryRow(ctx, `SELECT current_database()`).Scan(&dbName); err != nil {
		t.Fatalf("reading database name: %v", err)
	}
	if dbName == "freereps" {
		t.Fatalf("refusing to write to the database named %q — point FREEREPS_TEST_DSN at a scratch database", dbName)
	}

	// Only this user's rows are removed, so a scratch database shared with
	// other tests keeps its contents.
	if _, err := db.Pool.Exec(ctx, `DELETE FROM health_metrics WHERE user_id = $1`, aggTestUser); err != nil {
		t.Fatalf("clearing health_metrics: %v", err)
	}
	if _, err := db.Pool.Exec(ctx, `DELETE FROM source_priority WHERE user_id = $1`, aggTestUser); err != nil {
		t.Fatalf("clearing source_priority: %v", err)
	}
	return db
}

func qty(v float64) *float64 { return &v }

func at(hour, minute, second int) time.Time {
	return time.Date(2026, 9, 18, hour, minute, second, 0, time.UTC)
}

func row(t time.Time, metric, source string, v float64) models.HealthMetricRow {
	return models.HealthMetricRow{
		Time: t, UserID: aggTestUser, MetricName: metric,
		Source: source, Units: "count", Qty: qty(v),
	}
}

func insert(t *testing.T, db *DB, rows []models.HealthMetricRow) {
	t.Helper()
	if _, err := db.InsertHealthMetrics(context.Background(), rows); err != nil {
		t.Fatalf("inserting rows: %v", err)
	}
}

func nearly(got, want float64) bool { return math.Abs(got-want) < 0.001 }

// TestStepsAreNotCountedTwice is the regression test for the front page
// reporting 32361 steps for 2026-09-18 against the 16652 Apple Health had
// recorded. Oura writes the day's total as one row at noon; the reader chose a
// source per 5-minute window, so that total survived in its own window while
// the hourly Apple blocks were summed around it.
//
// The three read paths have to agree, which they did not: the metric page
// resolved the activity priority and the front page the _default one.
func TestStepsAreNotCountedTwice(t *testing.T) {
	db := aggTestDB(t)
	ctx := context.Background()

	// Apple Health reports hourly blocks; Oura one daily total, at noon.
	appleBlocks := []float64{152, 537.4, 520.2, 666.9, 2870.4, 4785.6, 668, 176, 797, 906.4, 4004.6, 264, 175}
	var want float64
	rows := make([]models.HealthMetricRow, 0, len(appleBlocks)+1)
	for i, v := range appleBlocks {
		rows = append(rows, row(at(5+i, 0, 0), "step_count", "", v))
		want += v
	}
	rows = append(rows, row(at(12, 0, 0), "step_count", "Oura", 15886))
	insert(t, db, rows)

	// The deployed configuration: Oura leads by default, Apple Health for activity.
	if err := db.UpsertSourcePriority(ctx, aggTestUser, "_default", []string{"Oura", "", "Withings"}); err != nil {
		t.Fatalf("setting _default priority: %v", err)
	}
	if err := db.UpsertSourcePriority(ctx, aggTestUser, "activity", []string{"", "Oura"}); err != nil {
		t.Fatalf("setting activity priority: %v", err)
	}

	start, end := at(0, 0, 0), at(0, 0, 0).AddDate(0, 0, 1)

	series, err := db.GetTimeSeries(ctx, "step_count", start, end, "1 day", aggTestUser)
	if err != nil {
		t.Fatalf("GetTimeSeries: %v", err)
	}
	if len(series) != 1 || series[0].Avg == nil || !nearly(*series[0].Avg, want) {
		t.Errorf("GetTimeSeries = %v, want one day of %.1f", series, want)
	}

	daily, err := db.GetDailySeries(ctx, aggTestUser, []string{"step_count"}, start, end)
	if err != nil {
		t.Fatalf("GetDailySeries: %v", err)
	}
	points := daily["step_count"]
	if len(points) != 1 || !nearly(points[0].Value, want) {
		t.Errorf("GetDailySeries = %v, want one day of %.1f — the front page and the metric page have to agree", points, want)
	}

	stats, err := db.GetMetricStats(ctx, "step_count", start, end, aggTestUser)
	if err != nil {
		t.Fatalf("GetMetricStats: %v", err)
	}
	if stats.Avg == nil || !nearly(*stats.Avg, want) {
		t.Errorf("GetMetricStats.Avg = %v, want the range total %.1f", stats.Avg, want)
	}
}

// TestCumulativeKeepsEverySample covers the report that brought this up: Health
// Auto Export can emit per-second samples for a counter, and the reader kept
// one row per 5-minute window, so a day of steps arrived as roughly a
// hundredth of itself.
func TestCumulativeKeepsEverySample(t *testing.T) {
	db := aggTestDB(t)
	ctx := context.Background()

	var want float64
	rows := make([]models.HealthMetricRow, 0, 300)
	for i := 0; i < 300; i++ {
		rows = append(rows, row(at(9, i/60, i%60), "step_count", "", 5))
		want += 5
	}
	insert(t, db, rows)

	start, end := at(0, 0, 0), at(0, 0, 0).AddDate(0, 0, 1)
	stats, err := db.GetMetricStats(ctx, "step_count", start, end, aggTestUser)
	if err != nil {
		t.Fatalf("GetMetricStats: %v", err)
	}
	if stats.Avg == nil || !nearly(*stats.Avg, want) {
		t.Errorf("GetMetricStats.Avg = %v, want %.0f from 300 samples", stats.Avg, want)
	}
	if stats.Count != 300 {
		t.Errorf("Count = %d, want 300", stats.Count)
	}
}

// TestHeartRateWeighsWindowsAlike pins the two-stage average. On 2026-09-18 six
// workouts covered 16% of the day and held 77% of the heart rate rows, so an
// average over rows is carried by the training while an average over 5-minute
// windows is not.
//
// It also pins that the source is resolved per window rather than per day: Oura
// leads for heart rate, and a daily choice would drop the 54 windows in which
// only Apple Health reported — the morning session among them.
func TestHeartRateWeighsWindowsAlike(t *testing.T) {
	db := aggTestDB(t)
	ctx := context.Background()

	insert(t, db, []models.HealthMetricRow{
		// 06:00 window — Apple Health only, three samples averaging 150.
		row(at(6, 0, 0), "heart_rate", "", 100),
		row(at(6, 1, 0), "heart_rate", "", 150),
		row(at(6, 2, 0), "heart_rate", "", 200),
		// 06:05 window — Apple Health only, one quiet sample.
		row(at(6, 5, 0), "heart_rate", "", 50),
	})
	if err := db.UpsertSourcePriority(ctx, aggTestUser, "_default", []string{"Oura", "", "Withings"}); err != nil {
		t.Fatalf("setting _default priority: %v", err)
	}

	start, end := at(0, 0, 0), at(0, 0, 0).AddDate(0, 0, 1)
	stats, err := db.GetMetricStats(ctx, "heart_rate", start, end, aggTestUser)
	if err != nil {
		t.Fatalf("GetMetricStats: %v", err)
	}

	// (150 + 50) / 2. Averaging the four rows directly gives 125, which is the
	// dense window outvoting the quiet one.
	if stats.Avg == nil || !nearly(*stats.Avg, 100) {
		t.Errorf("Avg = %v, want 100 (two windows weighed alike, not 125 over four rows)", stats.Avg)
	}
	// The extremes stay over the rows, so the range keeps the real values
	// rather than the range of the window averages (150 and 50).
	if stats.Min == nil || !nearly(*stats.Min, 50) {
		t.Errorf("Min = %v, want 50", stats.Min)
	}
	if stats.Max == nil || !nearly(*stats.Max, 200) {
		t.Errorf("Max = %v, want 200 — the extremes come from the rows, not the window averages", stats.Max)
	}
	if stats.Count != 4 {
		t.Errorf("Count = %d, want 4", stats.Count)
	}
}

// TestHeartRateKeepsWindowsTheLeadingSourceMisses is the other half of the
// source rule: Oura leads for heart rate, but a window Oura never reported
// belongs to Apple Health rather than to nobody.
//
// Unlike the tests above this one also passes on the previous code. It guards
// the property the rewrite could have removed: resolving the source per day,
// which is what cumulative metrics need, would hand the whole day to Oura and
// delete every window Oura missed.
func TestHeartRateKeepsWindowsTheLeadingSourceMisses(t *testing.T) {
	db := aggTestDB(t)
	ctx := context.Background()

	insert(t, db, []models.HealthMetricRow{
		// 07:00 — both sources report; Oura leads and Apple is discarded.
		row(at(7, 0, 0), "heart_rate", "Oura", 60),
		row(at(7, 1, 0), "heart_rate", "", 90),
		// 07:05 — Apple Health only; the window has to survive.
		row(at(7, 5, 0), "heart_rate", "", 80),
	})
	if err := db.UpsertSourcePriority(ctx, aggTestUser, "_default", []string{"Oura", "", "Withings"}); err != nil {
		t.Fatalf("setting _default priority: %v", err)
	}

	start, end := at(0, 0, 0), at(0, 0, 0).AddDate(0, 0, 1)
	stats, err := db.GetMetricStats(ctx, "heart_rate", start, end, aggTestUser)
	if err != nil {
		t.Fatalf("GetMetricStats: %v", err)
	}

	// (60 + 80) / 2. A daily source choice would report 60 and lose the second
	// window; keeping Apple's 90 in the first window would report 76.7.
	if stats.Avg == nil || !nearly(*stats.Avg, 70) {
		t.Errorf("Avg = %v, want 70", stats.Avg)
	}
	if stats.Count != 2 {
		t.Errorf("Count = %d, want 2 — Apple's competing row in the Oura window is dropped, its own window is not", stats.Count)
	}
}

// TestDirectIntegrationOutranksItsHealthKitCopy covers the heart rate the Oura
// app writes into HealthKit while the Oura API lags behind: on 2026-09-27 the
// API held nothing after 12:14:45Z for four hours. The copy arrives through the
// iOS app under the same source name, so the client rank decides — the API row
// wins its window, and the copy fills the window the API has not delivered.
func TestDirectIntegrationOutranksItsHealthKitCopy(t *testing.T) {
	db := aggTestDB(t)
	ctx := context.Background()

	copyRow := func(ts time.Time, v float64) models.HealthMetricRow {
		r := row(ts, "heart_rate", "Oura", v)
		r.Client = "freereps_ios"
		return r
	}
	insert(t, db, []models.HealthMetricRow{
		// 07:00 — the API and the HealthKit copy both report.
		row(at(7, 0, 0), "heart_rate", "Oura", 60),
		copyRow(at(7, 0, 30), 70),
		// 07:05 — the copy only; the API has not delivered this window yet.
		copyRow(at(7, 5, 0), 80),
	})
	if err := db.UpsertSourcePriority(ctx, aggTestUser, "_default", []string{"Oura", "", "Withings"}); err != nil {
		t.Fatalf("setting _default priority: %v", err)
	}

	start, end := at(0, 0, 0), at(0, 0, 0).AddDate(0, 0, 1)
	stats, err := db.GetMetricStats(ctx, "heart_rate", start, end, aggTestUser)
	if err != nil {
		t.Fatalf("GetMetricStats: %v", err)
	}

	// (60 + 80) / 2. Ranking the app's client first, as for Apple Health, would
	// report (70 + 80) / 2 = 75.
	if stats.Avg == nil || !nearly(*stats.Avg, 70) {
		t.Errorf("Avg = %v, want 70", stats.Avg)
	}
	if stats.Count != 2 {
		t.Errorf("Count = %d, want 2", stats.Count)
	}
}
