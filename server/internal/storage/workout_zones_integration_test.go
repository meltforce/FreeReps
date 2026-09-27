//go:build integration

package storage

import (
	"context"
	"testing"
	"time"

	"github.com/claude/freereps/internal/models"
)

// TestRestingHeartRateTakesOneSourcePerDay covers the property the zone edges
// depend on: Apple Health and Oura report different figures for the same day,
// and a median over both would sit between two definitions of the metric.
func TestRestingHeartRateTakesOneSourcePerDay(t *testing.T) {
	db := aggTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 27, 18, 0, 0, 0, time.UTC)

	day := func(d int) time.Time { return time.Date(2026, 9, d, 0, 0, 0, 0, time.UTC) }
	rows := []struct {
		t      time.Time
		source string
		bpm    float64
	}{
		{day(24).Add(5 * time.Hour), "", 70},
		{day(24).Add(12 * time.Hour), "Oura", 60},
		{day(25).Add(5 * time.Hour), "", 68},
		{day(25).Add(12 * time.Hour), "Oura", 58},
		// A day only Apple Health reported counts under either priority.
		{day(26).Add(5 * time.Hour), "", 72},
		// Outside the 30-day window.
		{time.Date(2026, 8, 20, 5, 0, 0, 0, time.UTC), "", 90},
	}
	for _, r := range rows {
		m := row(r.t, "resting_heart_rate", r.source, r.bpm)
		m.Units = "bpm"
		insert(t, db, []models.HealthMetricRow{m})
	}

	cases := []struct {
		priority []string
		want     float64
	}{
		{[]string{"Oura", ""}, 60},
		{[]string{"", "Oura"}, 70},
	}
	for _, c := range cases {
		if err := db.UpsertSourcePriority(ctx, aggTestUser, "_default", c.priority); err != nil {
			t.Fatalf("setting priority: %v", err)
		}
		got, err := db.restingHeartRate(ctx, aggTestUser, now)
		if err != nil {
			t.Fatalf("restingHeartRate: %v", err)
		}
		if got != c.want {
			t.Errorf("priority %q: got %v, want %v", c.priority, got, c.want)
		}
	}
}
