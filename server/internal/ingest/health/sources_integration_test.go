//go:build integration

package health

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// TestHealthKitCopiesOfDirectlySyncedProviders runs the source policy against
// the database: the Oura copy of a workout and its heart rate, as the Oura app
// wrote them into HealthKit on 2026-09-27, are dropped while Oura is
// connected and stored under "Oura" while it is not. Watch data in the same
// payload is stored as Apple Health either way.
func TestHealthKitCopiesOfDirectlySyncedProviders(t *testing.T) {
	db := sleepTestDB(t)
	ctx := context.Background()
	clear := func() {
		for _, stmt := range []string{
			`DELETE FROM health_metrics WHERE user_id = $1`,
			`DELETE FROM workouts WHERE user_id = $1`,
			`DELETE FROM category_samples WHERE user_id = $1`,
			`DELETE FROM oura_tokens WHERE user_id = $1`,
		} {
			if _, err := db.Pool.Exec(ctx, stmt, sleepTestUser); err != nil {
				t.Fatalf("clearing: %v", err)
			}
		}
	}
	clear()
	t.Cleanup(clear)

	payload := `{"data":{
	  "metrics":[{"name":"heart_rate","units":"count/min","data":[
	    {"date":"2026-09-27 08:10:00 +0000","qty":131,"source_bundle":"com.apple.health.5C1F0A77","source_name":"Linus Watch Ultra 2"},
	    {"date":"2026-09-27 08:10:02 +0000","qty":133,"source_bundle":"com.ouraring.oura","source_name":"Oura"}]}],
	  "workouts":[
	    {"id":"6ea3accf-579b-4648-966e-5bf62ba5e71d","name":"Running","start":"2026-09-27 08:08:32 +0000","end":"2026-09-27 08:32:46 +0000","duration":1454,
	     "source_bundle":"com.apple.health.5C1F0A77","source_name":"Linus Watch Ultra 2"},
	    {"id":"4b3ab45b-8bf2-43d7-aa31-554cb51a4227","name":"Running","start":"2026-09-27 08:08:00 +0000","end":"2026-09-27 08:33:00 +0000","duration":1500,
	     "source_bundle":"com.ouraring.oura","source_name":"Oura"}],
	  "category_samples":[
	    {"id":"0b8f7c2e-4a1d-4e6b-9c3f-2d5e8a7b1c90","type":"HKCategoryTypeIdentifierMindfulSession","value":0,
	     "start_date":"2026-09-27 06:00:00 +0000","end_date":"2026-09-27 06:10:00 +0000","source":"Oura","source_bundle":"com.ouraring.oura"}]}}`

	p := NewProvider(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	app := WithClient(ctx, ClientIOSApp)

	count := func(query string) int {
		t.Helper()
		var n int
		if err := db.Pool.QueryRow(ctx, query, sleepTestUser).Scan(&n); err != nil {
			t.Fatalf("counting: %v", err)
		}
		return n
	}
	check := func(label string, wantOura int) {
		t.Helper()
		if got := count(`SELECT count(*) FROM health_metrics WHERE user_id = $1 AND source = '' AND source_bundle = 'com.apple.health.5C1F0A77'`); got != 1 {
			t.Errorf("%s: %d Apple heart rate rows, want 1", label, got)
		}
		if got := count(`SELECT count(*) FROM workouts WHERE user_id = $1 AND source = ''`); got != 1 {
			t.Errorf("%s: %d Apple workouts, want 1", label, got)
		}
		for _, q := range []string{
			`SELECT count(*) FROM health_metrics WHERE user_id = $1 AND source = 'Oura'`,
			`SELECT count(*) FROM workouts WHERE user_id = $1 AND source = 'Oura'`,
			`SELECT count(*) FROM category_samples WHERE user_id = $1 AND source = 'Oura'`,
		} {
			if got := count(q); got != wantOura {
				t.Errorf("%s: %q = %d, want %d", label, q, got, wantOura)
			}
		}
	}

	// Oura connected: its copies are dropped.
	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO oura_tokens (user_id, client_id, client_secret, access_token) VALUES ($1, 'id', 'secret', 'token')`,
		sleepTestUser); err != nil {
		t.Fatalf("connecting Oura: %v", err)
	}
	res, err := p.Ingest(app, metricPayload(t, payload), sleepTestUser)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if res.SourceCopiesDropped != 3 {
		t.Errorf("dropped %d copies, want 3", res.SourceCopiesDropped)
	}
	check("connected", 0)

	// Oura not connected: HealthKit is the only path, so its data is kept.
	clear()
	if _, err := p.Ingest(app, metricPayload(t, payload), sleepTestUser); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	check("not connected", 1)
}
