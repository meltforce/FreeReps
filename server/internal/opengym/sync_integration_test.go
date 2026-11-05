//go:build integration

// The reconciliation's guarantees rest on SQL: the natural key of workout_sets,
// the delete by external id, the hash table's primary key, and the map join that
// supplies the catalog. The unit tests run against a fake store; this test runs
// the syncer against the schema the migrations build.
//
// Run with:
//
//	FREEREPS_TEST_DSN=postgres://user:pass@host:port/db?sslmode=disable \
//	  go test -tags integration ./internal/opengym/
package opengym

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/claude/freereps/internal/storage"
)

func integrationDB(t *testing.T) (*storage.DB, int) {
	t.Helper()
	dsn := os.Getenv("FREEREPS_TEST_DSN")
	if dsn == "" {
		t.Skip("FREEREPS_TEST_DSN not set")
	}
	migrations, err := filepath.Abs("../../migrations")
	if err != nil {
		t.Fatal(err)
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

	var name string
	if err := db.Pool.QueryRow(ctx, `SELECT current_database()`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name == "freereps" {
		t.Fatalf("refusing to write to the database named %q — point FREEREPS_TEST_DSN at a scratch database", name)
	}

	uid, err := db.GetOrCreateUser(ctx, "opengym-integration-test", "")
	if err != nil {
		t.Fatalf("creating user: %v", err)
	}
	clean := func() {
		ctx := context.Background()
		_ = db.DeleteOpenGymCredentials(ctx, uid)
		_, _ = db.Pool.Exec(ctx, `DELETE FROM workout_sets WHERE user_id = $1`, uid)
		_, _ = db.Pool.Exec(ctx, `DELETE FROM import_logs WHERE user_id = $1`, uid)
		_, _ = db.Pool.Exec(ctx, `DELETE FROM health_metrics WHERE user_id = $1`, uid)
	}
	clean()
	t.Cleanup(clean)
	return db, uid
}

func TestSyncAgainstSchema(t *testing.T) {
	db, uid := integrationDB(t)
	ctx := context.Background()

	syncFrom := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if err := db.UpsertOpenGymCredentials(ctx, uid, "https://gym.test", "tok-1", syncFrom); err != nil {
		t.Fatal(err)
	}

	api := &fakeAPI{rev: Rev{Rev: 3, WID: "w1"}, state: stateWithFixture(t), me: &Me{Token: "tok-2"}}
	s := NewSyncer(db, time.Minute, berlin(t), slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.newClient = func(string) API { return api }

	count := func() int {
		var n int
		if err := db.Pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM workout_sets WHERE user_id = $1 AND source = 'openGym'`, uid).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	s.SyncUser(ctx, uid)
	if n := count(); n != 14 {
		t.Fatalf("stored sets = %d, want 14", n)
	}
	creds, err := db.GetOpenGymCredentials(ctx, uid)
	if err != nil || creds.Token != "tok-2" {
		t.Errorf("token after renewal = %v, %v, want tok-2", creds, err)
	}
	state, err := db.GetOpenGymSyncState(ctx, uid)
	if err != nil || state == nil || state.Rev != 3 || state.WID != "w1" {
		t.Errorf("sync state = %+v, %v, want rev 3 wid w1", state, err)
	}

	// A new revision with the same workouts stores nothing and keeps the count.
	api.rev.Rev = 4
	s.SyncUser(ctx, uid)
	if n := count(); n != 14 {
		t.Errorf("stored sets after an unchanged state = %d, want 14", n)
	}
	var inserted int
	if err := db.Pool.QueryRow(ctx,
		`SELECT workouts_inserted FROM import_logs WHERE user_id = $1 AND source = $2
		 ORDER BY created_at DESC LIMIT 1`, uid, ImportSource).Scan(&inserted); err != nil {
		t.Fatal(err)
	}
	if inserted != 0 {
		t.Errorf("workouts_inserted on an unchanged state = %d, want 0", inserted)
	}

	// Deleting the workout in openGym removes its sets.
	st := stateWithFixture(t)
	kept := st.Workouts[:0]
	for _, w := range st.Workouts {
		if w.ID != "iwfp6ie5n94d" {
			kept = append(kept, w)
		}
	}
	st.Workouts = kept
	api.state, api.rev.Rev = st, 5
	s.SyncUser(ctx, uid)
	if n := count(); n != 4 {
		t.Errorf("stored sets after a deletion = %d, want 4", n)
	}

	// Rows carry the openGym id for the map join and no Hevy template id.
	var srcID, tplID string
	if err := db.Pool.QueryRow(ctx,
		`SELECT source_exercise_id, exercise_template_id FROM workout_sets
		 WHERE user_id = $1 AND source = 'openGym' LIMIT 1`, uid).Scan(&srcID, &tplID); err != nil {
		t.Fatal(err)
	}
	if srcID != "1326" || tplID != "" {
		t.Errorf("source_exercise_id/exercise_template_id = %q/%q, want 1326/empty", srcID, tplID)
	}
}
