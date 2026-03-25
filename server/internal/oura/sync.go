package oura

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/claude/freereps/internal/config"
	"github.com/claude/freereps/internal/models"
	"github.com/claude/freereps/internal/storage"
)

// dataTypes defines the Oura data types to sync, in order.
var dataTypes = []string{
	"daily_readiness",
	"daily_sleep",
	"sleep",
	"daily_activity",
	"heartrate",
	"daily_spo2",
	"daily_stress",
	"daily_resilience",
	"daily_cardiovascular_age",
	"vo2_max",
	"workout",
}

// Syncer polls the Oura API and stores data in FreeReps.
type Syncer struct {
	client   *Client
	tokenMgr *TokenManager
	db       *storage.DB
	cfg      config.OuraConfig
	log      *slog.Logger
}

// NewSyncer creates a new Oura sync orchestrator.
func NewSyncer(client *Client, tokenMgr *TokenManager, db *storage.DB, cfg config.OuraConfig, log *slog.Logger) *Syncer {
	return &Syncer{
		client:   client,
		tokenMgr: tokenMgr,
		db:       db,
		cfg:      cfg,
		log:      log,
	}
}

// Run starts the polling loop. Blocks until ctx is cancelled.
func (s *Syncer) Run(ctx context.Context) {
	// Sync immediately on startup.
	if err := s.SyncOnce(ctx); err != nil {
		s.log.Error("initial oura sync failed", "error", err)
	}

	ticker := time.NewTicker(s.cfg.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("oura sync stopped")
			return
		case <-ticker.C:
			if err := s.SyncOnce(ctx); err != nil {
				s.log.Error("oura sync cycle failed", "error", err)
			}
		}
	}
}

// SyncOnce performs one full sync cycle for all users with Oura tokens.
func (s *Syncer) SyncOnce(ctx context.Context) error {
	users, err := s.db.ListOuraTokenUsers(ctx)
	if err != nil {
		return fmt.Errorf("listing oura users: %w", err)
	}
	if len(users) == 0 {
		return nil
	}

	s.log.Info("oura sync starting", "users", len(users))
	for _, uid := range users {
		if err := s.SyncUser(ctx, uid); err != nil {
			s.log.Error("oura sync failed for user", "user_id", uid, "error", err)
		}
	}
	s.log.Info("oura sync complete")
	return nil
}

// SyncUser syncs all data types for a specific user.
func (s *Syncer) SyncUser(ctx context.Context, userID int) error {
	token, err := s.tokenMgr.GetValidToken(ctx, userID)
	if err != nil {
		return fmt.Errorf("getting token: %w", err)
	}

	for _, dt := range dataTypes {
		if err := s.syncDataType(ctx, userID, token, dt); err != nil {
			s.log.Error("oura sync data type failed", "user_id", userID, "data_type", dt, "error", err)
			// Continue with other data types even if one fails.
		}
	}
	return nil
}

// TriggerSync performs an immediate sync for a specific user (manual sync button).
func (s *Syncer) TriggerSync(ctx context.Context, userID int) error {
	return s.SyncUser(ctx, userID)
}

// syncDataType fetches and stores one data type for a user.
func (s *Syncer) syncDataType(ctx context.Context, userID int, token, dataType string) error {
	// Determine date range: from last sync (or backfill) to today.
	endDate := time.Now().Format("2006-01-02")

	state, err := s.db.GetOuraSyncState(ctx, userID, dataType)
	if err != nil {
		return fmt.Errorf("getting sync state: %w", err)
	}

	var startDate string
	if state != nil {
		startDate = state.LastSync.Format("2006-01-02")
	} else {
		startDate = time.Now().AddDate(0, 0, -s.cfg.BackfillDays).Format("2006-01-02")
	}

	if err := s.fetchAndStore(ctx, userID, token, dataType, startDate, endDate); err != nil {
		return err
	}

	// Update sync state.
	now := time.Now()
	return s.db.UpsertOuraSyncState(ctx, userID, dataType, now)
}

// fetchAndStore dispatches to the appropriate fetch+map+insert logic per data type.
func (s *Syncer) fetchAndStore(ctx context.Context, userID int, token, dataType, startDate, endDate string) error {
	switch dataType {
	case "daily_readiness":
		items, err := s.client.GetDailyReadiness(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		rows := MapDailyReadiness(items, userID)
		_, err = s.db.InsertHealthMetrics(ctx, rows)
		return err

	case "daily_sleep":
		items, err := s.client.GetDailySleep(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		rows := MapDailySleep(items, userID)
		_, err = s.db.InsertHealthMetrics(ctx, rows)
		return err

	case "daily_activity":
		items, err := s.client.GetDailyActivity(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		rows := MapDailyActivity(items, userID)
		_, err = s.db.InsertHealthMetrics(ctx, rows)
		return err

	case "sleep":
		items, err := s.client.GetSleep(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		sessions, stages := MapSleepSessions(items, userID)
		for _, session := range sessions {
			if err := s.db.InsertSleepSession(ctx, session); err != nil {
				s.log.Warn("inserting oura sleep session", "error", err)
			}
		}
		if len(stages) > 0 {
			if _, err := s.db.InsertSleepStages(ctx, stages); err != nil {
				return err
			}
		}
		// Also insert overlapping metrics from sleep data.
		s.insertSleepMetrics(ctx, items, userID)
		return nil

	case "heartrate":
		// Heartrate uses datetime params, not date params.
		startDT := startDate + "T00:00:00+00:00"
		endDT := endDate + "T23:59:59+00:00"
		items, err := s.client.GetHeartRate(ctx, token, startDT, endDT)
		if err != nil {
			return err
		}
		rows := MapHeartRate(items, userID)
		_, err = s.db.InsertHealthMetrics(ctx, rows)
		return err

	case "daily_spo2":
		items, err := s.client.GetDailySpO2(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		rows := MapDailySpO2(items, userID)
		_, err = s.db.InsertHealthMetrics(ctx, rows)
		return err

	case "daily_stress":
		items, err := s.client.GetDailyStress(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		rows := MapDailyStress(items, userID)
		_, err = s.db.InsertHealthMetrics(ctx, rows)
		return err

	case "daily_resilience":
		items, err := s.client.GetDailyResilience(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		rows := MapDailyResilience(items, userID)
		_, err = s.db.InsertHealthMetrics(ctx, rows)
		return err

	case "daily_cardiovascular_age":
		items, err := s.client.GetDailyCardiovascularAge(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		rows := MapDailyCardiovascularAge(items, userID)
		_, err = s.db.InsertHealthMetrics(ctx, rows)
		return err

	case "vo2_max":
		items, err := s.client.GetVO2Max(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		rows := MapVO2Max(items, userID)
		_, err = s.db.InsertHealthMetrics(ctx, rows)
		return err

	case "workout":
		items, err := s.client.GetWorkouts(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		workouts := MapWorkouts(items, userID)
		for _, w := range workouts {
			if _, err := s.db.InsertWorkout(ctx, w); err != nil {
				s.log.Warn("inserting oura workout", "error", err)
			}
		}
		return nil

	default:
		return fmt.Errorf("unknown data type: %s", dataType)
	}
}

// insertSleepMetrics extracts overlapping health metrics from detailed sleep data
// (resting HR, HRV, respiratory rate).
func (s *Syncer) insertSleepMetrics(ctx context.Context, items []SleepItem, userID int) {
	var rows []models.HealthMetricRow
	for _, item := range items {
		if item.Type == "deleted" {
			continue
		}
		t := parseDay(item.Day)
		if item.LowestHeartRate != nil {
			rows = append(rows, metricRow(t, userID, "resting_heart_rate", "bpm", intToFloat(*item.LowestHeartRate)))
		}
		if item.AverageHRV != nil {
			rows = append(rows, metricRow(t, userID, "heart_rate_variability", "ms", intToFloat(*item.AverageHRV)))
		}
		if item.AverageBreath != nil {
			// Oura reports breaths/second; convert to breaths/minute.
			bpm := *item.AverageBreath * 60
			rows = append(rows, metricRow(t, userID, "respiratory_rate", "breaths/min", floatPtr(bpm)))
		}
	}
	if len(rows) > 0 {
		if _, err := s.db.InsertHealthMetrics(ctx, rows); err != nil {
			s.log.Warn("inserting oura sleep metrics", "error", err)
		}
	}
}
