//go:build integration

// IsMetricAllowed decides per ingest row whether a metric is stored, and it now
// combines two switches: metric_allowlist.enabled for the whole server and
// user_metric_enabled for one user. A mistake in the join either stores what a
// user disabled or rejects a metric for every user because one user disabled
// it; both need the real tables to show.
//
// Run with:
//
//	FREEREPS_TEST_DSN=postgres://user:pass@host:port/db?sslmode=disable \
//	  go test -tags integration ./internal/storage/
package storage

import (
	"context"
	"testing"
)

const (
	enabledTestUser  = 4244
	enabledOtherUser = 4245
)

func TestMetricEnabledIsPerUser(t *testing.T) {
	db := aggTestDB(t)
	ctx := context.Background()
	for _, uid := range []int{enabledTestUser, enabledOtherUser} {
		if _, err := db.Pool.Exec(ctx, `DELETE FROM user_metric_enabled WHERE user_id = $1`, uid); err != nil {
			t.Fatalf("clearing rows: %v", err)
		}
	}

	allowed := func(uid int, name string) bool {
		t.Helper()
		ok, err := db.IsMetricAllowed(ctx, uid, name)
		if err != nil {
			t.Fatalf("IsMetricAllowed(%d, %s): %v", uid, name, err)
		}
		return ok
	}

	if !allowed(enabledTestUser, "dietary_caffeine") {
		t.Fatal("a metric without a user row must follow the server-wide default")
	}
	if allowed(enabledTestUser, "not_a_metric") {
		t.Fatal("a metric outside the allowlist must be rejected")
	}

	if err := db.SaveMetricEnabled(ctx, enabledTestUser, map[string]bool{"dietary_caffeine": false}); err != nil {
		t.Fatalf("SaveMetricEnabled: %v", err)
	}
	if allowed(enabledTestUser, "dietary_caffeine") {
		t.Fatal("a metric the user disabled must be rejected for that user")
	}
	if !allowed(enabledOtherUser, "dietary_caffeine") {
		t.Fatal("disabling a metric for one user must not reject it for another")
	}

	list, err := db.GetUserAllowlist(ctx, enabledTestUser)
	if err != nil {
		t.Fatalf("GetUserAllowlist: %v", err)
	}
	for _, m := range list {
		if m.MetricName == "dietary_caffeine" && m.Enabled {
			t.Fatal("GetUserAllowlist must report the user's override")
		}
	}

	if err := db.SaveMetricEnabled(ctx, enabledTestUser, map[string]bool{"dietary_caffeine": true}); err != nil {
		t.Fatalf("SaveMetricEnabled: %v", err)
	}
	if !allowed(enabledTestUser, "dietary_caffeine") {
		t.Fatal("re-enabling a metric must accept it again")
	}
}
