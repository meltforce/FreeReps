//go:build integration

// Whether the ingest endpoint writes sleep is decided against the user's
// configured source priority, which lives in the database. A unit test with a
// fake store would assert the fake's behaviour, so this runs against a real
// PostgreSQL server.
//
// Run with:
//
//	FREEREPS_TEST_DSN=postgres://user:pass@host:port/db?sslmode=disable \
//	  go test -tags integration ./internal/ingest/health/
package health

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/claude/freereps/internal/models"
	"github.com/claude/freereps/internal/storage"
)

const sleepTestUser = 4243

func sleepTestDB(t *testing.T) *storage.DB {
	t.Helper()
	dsn := os.Getenv("FREEREPS_TEST_DSN")
	if dsn == "" {
		t.Skip("FREEREPS_TEST_DSN not set")
	}

	migrations, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	if err := storage.RunMigrations(dsn, migrations); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	ctx := context.Background()
	db, err := storage.New(ctx, dsn)
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

	for _, stmt := range []string{
		`DELETE FROM sleep_stages WHERE user_id = $1`,
		`DELETE FROM category_samples WHERE user_id = $1`,
		`DELETE FROM source_priority WHERE user_id = $1`,
	} {
		if _, err := db.Pool.Exec(ctx, stmt, sleepTestUser); err != nil {
			t.Fatalf("clearing: %v", err)
		}
	}
	return db
}

// sleepCategoryPayload is one night's worth of the shape the FreeReps iOS app
// sends: HealthKit category samples whose source names the app that wrote them
// into HealthKit — for an Oura wearer, "Oura".
func sleepCategoryPayload(t *testing.T) *models.HealthPayload {
	t.Helper()
	raw := `{"data":{"category_samples":[
	  {"id":"11111111-1111-4111-8111-111111111111","type":"HKCategoryTypeIdentifierSleepAnalysis",
	   "value":0,"value_label":"In Bed","start_date":"2026-09-19 23:53:29 +0200","end_date":"2026-09-20 08:29:59 +0200","source":"Oura"},
	  {"id":"22222222-2222-4222-8222-222222222222","type":"HKCategoryTypeIdentifierSleepAnalysis",
	   "value":2,"value_label":"Awake","start_date":"2026-09-19 23:53:29 +0200","end_date":"2026-09-20 00:01:59 +0200","source":"Oura"},
	  {"id":"33333333-3333-4333-8333-333333333333","type":"HKCategoryTypeIdentifierSleepAnalysis",
	   "value":3,"value_label":"Asleep Core","start_date":"2026-09-20 00:01:59 +0200","end_date":"2026-09-20 00:22:59 +0200","source":"Oura"}
	]}}`
	var p models.HealthPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("parsing payload: %v", err)
	}
	return &p
}

func countSleepStages(t *testing.T, db *storage.DB) int {
	t.Helper()
	var n int
	err := db.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM sleep_stages WHERE user_id = $1`, sleepTestUser).Scan(&n)
	if err != nil {
		t.Fatalf("counting stages: %v", err)
	}
	return n
}

// TestIngestSkipsSleepWhenAProviderSyncsItCovers the duplication of 2026-09-20:
// the Oura API and HealthKit both delivered the same night, with bounds that
// differ enough for the unique index to accept both, and the stage composition
// reported 17.27 hours against 8.6 hours in bed.
func TestIngestSkipsSleepWhenAProviderSyncsIt(t *testing.T) {
	db := sleepTestDB(t)
	ctx := context.Background()
	p := NewProvider(db, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := db.UpsertSourcePriority(ctx, sleepTestUser, "sleep", []string{"Oura", ""}); err != nil {
		t.Fatalf("setting priority: %v", err)
	}

	if _, err := p.Ingest(ctx, sleepCategoryPayload(t), sleepTestUser); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	if n := countSleepStages(t, db); n != 0 {
		t.Errorf("wrote %d sleep stages, want 0 — the Oura sync already delivers this night", n)
	}

	// The samples themselves are still stored; only the second copy of the
	// night's stages is refused.
	var samples int
	err := db.Pool.QueryRow(ctx,
		`SELECT count(*) FROM category_samples WHERE user_id = $1`, sleepTestUser).Scan(&samples)
	if err != nil {
		t.Fatalf("counting samples: %v", err)
	}
	if samples != 3 {
		t.Errorf("stored %d category samples, want 3 — only the stage extraction is skipped", samples)
	}
}

// TestIngestWritesSleepWhenNoProviderClaimsIt is the other half: without a
// syncing provider this endpoint is the only route sleep has, and refusing it
// would lose the data rather than deduplicate it.
func TestIngestWritesSleepWhenNoProviderClaimsIt(t *testing.T) {
	db := sleepTestDB(t)
	ctx := context.Background()
	p := NewProvider(db, slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Apple Health leads: no provider syncs sleep on its own.
	if err := db.UpsertSourcePriority(ctx, sleepTestUser, "_default", []string{"", "Oura"}); err != nil {
		t.Fatalf("setting priority: %v", err)
	}

	if _, err := p.Ingest(ctx, sleepCategoryPayload(t), sleepTestUser); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	// Three samples in, three stages out — "In Bed" included, because storing
	// it is not what broke the hypnogram; drawing it as a stage was.
	if n := countSleepStages(t, db); n != 3 {
		t.Errorf("wrote %d sleep stages, want 3 — this endpoint is the only route", n)
	}
}

// TestSleepCategoryRuleOutranksTheDefault pins that an explicit sleep rule wins:
// a user who leads with Oura by default but names Apple Health for sleep keeps
// their sleep from this endpoint.
func TestSleepCategoryRuleOutranksTheDefault(t *testing.T) {
	db := sleepTestDB(t)
	ctx := context.Background()
	p := NewProvider(db, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := db.UpsertSourcePriority(ctx, sleepTestUser, "_default", []string{"Oura", ""}); err != nil {
		t.Fatalf("setting default priority: %v", err)
	}
	if err := db.UpsertSourcePriority(ctx, sleepTestUser, "sleep", []string{"", "Oura"}); err != nil {
		t.Fatalf("setting sleep priority: %v", err)
	}

	if _, err := p.Ingest(ctx, sleepCategoryPayload(t), sleepTestUser); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if n := countSleepStages(t, db); n != 3 {
		t.Errorf("wrote %d sleep stages, want 3 — the sleep rule names Apple Health", n)
	}
}

// TestSleepDefaultPriorityDoesNotBlockIngest pins the fix for the bug where the
// global default ["Withings", "Oura", ""] caused sleepClaimedBySync to return
// true for users who have neither provider. Only an explicit sleep rule should
// be able to claim sleep for a syncing provider.
func TestSleepDefaultPriorityDoesNotBlockIngest(t *testing.T) {
	db := sleepTestDB(t)
	ctx := context.Background()
	p := NewProvider(db, slog.New(slog.NewTextHandler(io.Discard, nil)))

	db.SourcePriority = []string{"Withings", "Oura", ""}

	if _, err := p.Ingest(ctx, sleepCategoryPayload(t), sleepTestUser); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if n := countSleepStages(t, db); n != 3 {
		t.Errorf("wrote %d sleep stages, want 3 — the global default must not block sleep", n)
	}
}
