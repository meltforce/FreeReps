package server

import (
	"testing"
	"time"
)

// TestOuraNudgeStartsOncePerInterval: one app sync posts many batches, and
// only the first of them within ouraNudgeInterval starts an Oura sync.
func TestOuraNudgeStartsOncePerInterval(t *testing.T) {
	var n ouraNudge
	t0 := time.Date(2026, 9, 28, 11, 28, 0, 0, time.UTC)

	if !n.due(2, t0) {
		t.Fatal("first ingest does not start a sync")
	}
	if n.due(2, t0.Add(90*time.Second)) {
		t.Error("a second batch of the same app sync starts another sync")
	}
	if !n.due(3, t0.Add(90*time.Second)) {
		t.Error("another user's ingest is held back by the first user's sync")
	}
	if !n.due(2, t0.Add(ouraNudgeInterval)) {
		t.Error("an ingest after the interval does not start a sync")
	}
}
