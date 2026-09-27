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
