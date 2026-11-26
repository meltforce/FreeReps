//go:build integration

package health

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/claude/freereps/internal/models"
	"github.com/claude/freereps/internal/storage"
)

func metricPayload(t *testing.T, raw string) *models.HealthPayload {
	t.Helper()
	var p models.HealthPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("parsing payload: %v", err)
	}
	return &p
}

func checkpointsByItem(t *testing.T, db *storage.DB, client string) map[string]time.Time {
	t.Helper()
	cps, err := db.IngestCheckpoints(context.Background(), sleepTestUser, client)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]time.Time{}
	for _, c := range cps {
		out[c.Domain+"/"+c.Item] = c.NewestSample
	}
	return out
}

// TestCheckpointNeverMovesBack catches the two ways a checkpoint would make the
// iOS app skip data after a reinstall: a re-sent older window pulling it back is
// harmless, but a single checkpoint per domain rather than per metric would let
// a metric whose backfill finished early stand in for one that never started,
// and a checkpoint shared between clients would let Health Auto Export's
// deliveries stand in for the app's.
func TestCheckpointNeverMovesBack(t *testing.T) {
	db := sleepTestDB(t)
	ctx := context.Background()
	for _, stmt := range []string{
		`DELETE FROM health_metrics WHERE user_id = $1`,
		`DELETE FROM ingest_checkpoints WHERE user_id = $1`,
	} {
		if _, err := db.Pool.Exec(ctx, stmt, sleepTestUser); err != nil {
			t.Fatalf("clearing: %v", err)
		}
	}
	p := NewProvider(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	app := WithClient(ctx, ClientIOSApp)

	if _, err := p.Ingest(app, metricPayload(t, `{"data":{"metrics":[
	  {"name":"step_count","units":"count","data":[
	    {"date":"2026-09-20 10:00:00 +0000","qty":500},
	    {"date":"2026-09-20 11:00:00 +0000","qty":700}]},
	  {"name":"heart_rate","units":"count/min","data":[
	    {"date":"2026-09-20 09:00:00 +0000","Min":60,"Avg":65,"Max":70}]}]}}`), sleepTestUser); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	// A re-sent older window, as the daily 24-hour resend produces.
	if _, err := p.Ingest(app, metricPayload(t, `{"data":{"metrics":[
	  {"name":"step_count","units":"count","data":[{"date":"2026-09-20 08:00:00 +0000","qty":300}]}]}}`), sleepTestUser); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	// Health Auto Export delivers later data under its own client.
	if _, err := p.Ingest(ctx, metricPayload(t, `{"data":{"metrics":[
	  {"name":"step_count","units":"count","data":[{"date":"2026-09-20 12:00:00 +0000","qty":900}]}]}}`), sleepTestUser); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	got := checkpointsByItem(t, db, ClientIOSApp)
	want := map[string]time.Time{
		"metrics/step_count": time.Date(2026, 9, 20, 11, 0, 0, 0, time.UTC),
		"metrics/heart_rate": time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC),
	}
	for k, w := range want {
		if !got[k].Equal(w) {
			t.Errorf("app checkpoint %s = %s, want %s", k, got[k], w)
		}
	}
	if hae := checkpointsByItem(t, db, ClientHAE)["metrics/step_count"]; !hae.Equal(time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("hae checkpoint = %s, want 12:00", hae)
	}
}

// TestCheckpointRowCount: the row count the iOS app shows must count stored
// rows. A re-sent or changed row is not a new one, and rows that predate the
// counter are picked up by the recount.
func TestCheckpointRowCount(t *testing.T) {
	db := sleepTestDB(t)
	ctx := context.Background()
	for _, stmt := range []string{
		`DELETE FROM health_metrics WHERE user_id = $1`,
		`DELETE FROM ingest_checkpoints WHERE user_id = $1`,
	} {
		if _, err := db.Pool.Exec(ctx, stmt, sleepTestUser); err != nil {
			t.Fatalf("clearing: %v", err)
		}
	}
	p := NewProvider(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	app := WithClient(ctx, ClientIOSApp)
	send := func(raw string) {
		t.Helper()
		if _, err := p.Ingest(app, metricPayload(t, raw), sleepTestUser); err != nil {
			t.Fatalf("ingest: %v", err)
		}
	}
	count := func() *int64 {
		t.Helper()
		cps, err := db.IngestCheckpoints(ctx, sleepTestUser, ClientIOSApp)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range cps {
			if c.Domain == DomainMetrics && c.Item == "step_count" {
				return c.RowCount
			}
		}
		t.Fatal("no step_count checkpoint")
		return nil
	}

	// Rows stored years apart, before the checkpoint has a count.
	send(`{"data":{"metrics":[{"name":"step_count","units":"count","data":[
	  {"date":"2019-03-01 10:00:00 +0000","qty":1},
	  {"date":"2026-09-20 10:00:00 +0000","qty":500}]}]}}`)
	if n := count(); n != nil {
		t.Fatalf("a new checkpoint has a count of %d before the recount", *n)
	}
	if err := db.RecountIngestCheckpoints(ctx); err != nil {
		t.Fatal(err)
	}
	if n := count(); n == nil || *n != 2 {
		t.Fatalf("recount = %v, want 2", n)
	}

	// One changed row, one identical row, one new row.
	send(`{"data":{"metrics":[{"name":"step_count","units":"count","data":[
	  {"date":"2019-03-01 10:00:00 +0000","qty":1},
	  {"date":"2026-09-20 10:00:00 +0000","qty":900},
	  {"date":"2026-09-20 11:00:00 +0000","qty":700}]}]}}`)
	if n := count(); n == nil || *n != 3 {
		t.Fatalf("after a resend with one new row = %v, want 3", n)
	}
}
