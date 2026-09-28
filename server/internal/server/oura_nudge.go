package server

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/claude/freereps/internal/ingest/health"
)

// ouraNudgeInterval is the least time between two Oura syncs started by iOS app
// ingests of one user. One app sync posts dozens of batches within a minute or
// two; the first starts the Oura sync, the rest fall inside the interval.
const ouraNudgeInterval = 5 * time.Minute

// ouraNudge remembers when an app ingest last started an Oura sync, per user.
// The zero value is ready to use.
type ouraNudge struct {
	mu   sync.Mutex
	last map[int]time.Time
}

// due reports whether an Oura sync may start for uid at now, and records it if
// so.
func (n *ouraNudge) due(uid int, now time.Time) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.last == nil {
		n.last = map[int]time.Time{}
	}
	if last, ok := n.last[uid]; ok && now.Sub(last) < ouraNudgeInterval {
		return false
	}
	n.last[uid] = now
	return true
}

// nudgeOuraSync starts an Oura sync after an ingest from the iOS app.
//
// The Oura API lists a workout within minutes of its end, while the scheduled
// Oura sync runs every 30 minutes. When the Oura app has written a session into
// HealthKit it has also uploaded it, so an app sync is the moment the API holds
// something new. Starting the Oura sync then stores the workout and derives its
// heart rate from the HealthKit copies the app delivers in the same sync —
// whichever of the two arrives second fills the series
// (storage.FillSourceWorkoutsHeartRate). On 2026-09-28 a Yoga session waited
// from the app sync at 11:28Z to a manually started Oura sync at 11:37Z for its
// heart rate. Importing the HealthKit copy of the workout instead was rejected
// in DECISIONS.md, 2026-09-27.
func (s *Server) nudgeOuraSync(uid int) {
	if s.ouraSyncer == nil || !s.ouraNudge.due(uid, time.Now()) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		providers, err := s.db.DirectSyncProviders(ctx, uid)
		if err != nil {
			s.log.Warn("checking the Oura connection after an app ingest", "user_id", uid, "error", err)
			return
		}
		if !slices.Contains(providers, "Oura") {
			return
		}
		s.log.Info("starting an Oura sync after an iOS app ingest", "user_id", uid)
		if err := s.ouraSyncer.TriggerSync(ctx, uid); err != nil {
			s.log.Error("oura sync after an app ingest failed", "user_id", uid, "error", err)
		}
	}()
}

// isAppIngest reports whether the ingest came from the iOS app.
func isAppIngest(client string) bool {
	return client == health.ClientIOSApp
}
