package oura

import (
	"context"
	"encoding/json"
	"errors"
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

// syncStats tracks counts across a full sync cycle for import logging.
type syncStats struct {
	metricsReceived  int
	metricsInserted  int64
	workoutsReceived int
	workoutsInserted int
	sleepSessions    int
	errors           []string
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
		s.SyncUser(ctx, uid)
	}
	s.log.Info("oura sync complete")
	return nil
}

// SyncUser syncs all data types for a specific user and writes an import log.
func (s *Syncer) SyncUser(ctx context.Context, userID int) {
	start := time.Now()
	stats := &syncStats{}

	token, err := s.tokenMgr.GetValidToken(ctx, userID)
	if err != nil {
		s.logImport(ctx, userID, start, stats, fmt.Errorf("getting token: %w", err))
		return
	}

	var syncErr error
	for _, dt := range dataTypes {
		if err := s.syncDataType(ctx, userID, token, dt, stats); err != nil {
			// 404 or 401/403 are expected for endpoints not available for
			// the user's ring model or missing scopes — log as debug, not error.
			var apiErr *APIError
			if errors.As(err, &apiErr) && (apiErr.IsNotFound() || apiErr.IsUnauthorized()) {
				s.log.Debug("oura endpoint unavailable", "data_type", dt, "status", apiErr.StatusCode)
				continue
			}
			stats.errors = append(stats.errors, fmt.Sprintf("%s: %s", dt, err))
			s.log.Error("oura sync data type failed", "user_id", userID, "data_type", dt, "error", err)
			// Continue with other data types even if one fails.
		}
	}

	if len(stats.errors) > 0 {
		syncErr = fmt.Errorf("%d data type(s) failed", len(stats.errors))
	}
	s.db.InvalidateAvailableMetrics(userID)
	s.logImport(ctx, userID, start, stats, syncErr)
}

// TriggerSync performs an immediate sync for a specific user (manual sync button).
func (s *Syncer) TriggerSync(ctx context.Context, userID int) error {
	s.SyncUser(ctx, userID)
	return nil
}

// logImport writes an import log entry for an Oura sync cycle.
func (s *Syncer) logImport(ctx context.Context, userID int, start time.Time, stats *syncStats, syncErr error) {
	durationMs := int(time.Since(start).Milliseconds())
	status := "success"
	var errMsg *string

	if syncErr != nil {
		status = "error"
		msg := syncErr.Error()
		errMsg = &msg
	}

	var metadata *json.RawMessage
	if len(stats.errors) > 0 {
		raw, _ := json.Marshal(map[string]any{"data_type_errors": stats.errors})
		rm := json.RawMessage(raw)
		metadata = &rm
	}

	if _, err := s.db.InsertImportLog(ctx, storage.ImportLog{
		UserID:           userID,
		Source:           "oura_sync",
		Status:           status,
		MetricsReceived:  stats.metricsReceived,
		MetricsInserted:  stats.metricsInserted,
		WorkoutsReceived: stats.workoutsReceived,
		WorkoutsInserted: stats.workoutsInserted,
		SleepSessions:    stats.sleepSessions,
		DurationMs:       &durationMs,
		ErrorMessage:     errMsg,
		Metadata:         metadata,
	}); err != nil {
		s.log.Error("failed to log oura import", "error", err)
	}
}

// syncDataType fetches and stores one data type for a user.
func (s *Syncer) syncDataType(ctx context.Context, userID int, token, dataType string, stats *syncStats) error {
	// Determine date range: from last sync (or backfill) to today.
	// Always overlap by 2 days so delayed data (e.g. Oura sleep that appears
	// hours after waking) is picked up on subsequent syncs.
	endDate := time.Now().Format("2006-01-02")

	state, err := s.db.GetOuraSyncState(ctx, userID, dataType)
	if err != nil {
		return fmt.Errorf("getting sync state: %w", err)
	}

	var startDate string
	if state != nil {
		startDate = state.LastSync.AddDate(0, 0, -2).Format("2006-01-02")
	} else {
		startDate = time.Now().AddDate(0, 0, -s.cfg.BackfillDays).Format("2006-01-02")
	}

	if err := s.fetchAndStore(ctx, userID, token, dataType, startDate, endDate, stats); err != nil {
		return err
	}

	// Update sync state.
	now := time.Now()
	return s.db.UpsertOuraSyncState(ctx, userID, dataType, now)
}

// insertMetrics is a helper that inserts health metrics and updates stats.
func (s *Syncer) insertMetrics(ctx context.Context, rows []models.HealthMetricRow, stats *syncStats) error {
	stats.metricsReceived += len(rows)
	if len(rows) == 0 {
		return nil
	}
	inserted, err := s.db.InsertHealthMetrics(ctx, rows)
	if err != nil {
		return err
	}
	stats.metricsInserted += inserted
	return nil
}

// fetchAndStore dispatches to the appropriate fetch+map+insert logic per data type.
func (s *Syncer) fetchAndStore(ctx context.Context, userID int, token, dataType, startDate, endDate string, stats *syncStats) error {
	switch dataType {
	case "daily_readiness":
		items, err := s.client.GetDailyReadiness(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		return s.insertMetrics(ctx, MapDailyReadiness(items, userID), stats)

	case "daily_sleep":
		items, err := s.client.GetDailySleep(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		return s.insertMetrics(ctx, MapDailySleep(items, userID), stats)

	case "daily_activity":
		items, err := s.client.GetDailyActivity(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		return s.insertMetrics(ctx, MapDailyActivity(items, userID), stats)

	case "sleep":
		items, err := s.client.GetSleep(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		sessions, stages := MapSleepSessions(items, userID)
		for _, session := range sessions {
			if err := s.db.InsertSleepSession(ctx, session); err != nil {
				s.log.Warn("inserting oura sleep session", "error", err)
			} else {
				stats.sleepSessions++
			}
		}
		if len(stages) > 0 {
			if _, err := s.db.InsertSleepStages(ctx, stages); err != nil {
				return err
			}
		}
		// Insert sleep_analysis metrics for each long_sleep session so the
		// dashboard chart has data. Use noon UTC for stable dedup.
		if err := s.insertSleepAnalysis(ctx, sessions, stats); err != nil {
			return err
		}
		// Also insert overlapping metrics from sleep data.
		return s.insertSleepMetrics(ctx, items, userID, stats)

	case "heartrate":
		// Oura limits heartrate queries to 30 days max. Chunk accordingly.
		start, _ := time.Parse("2006-01-02", startDate)
		end, _ := time.Parse("2006-01-02", endDate)
		for chunkStart := start; chunkStart.Before(end); {
			chunkEnd := chunkStart.AddDate(0, 0, 30)
			if chunkEnd.After(end) {
				chunkEnd = end
			}
			startDT := chunkStart.Format("2006-01-02") + "T00:00:00+00:00"
			endDT := chunkEnd.Format("2006-01-02") + "T23:59:59+00:00"
			items, err := s.client.GetHeartRate(ctx, token, startDT, endDT)
			if err != nil {
				return err
			}
			if err := s.insertMetrics(ctx, MapHeartRate(items, userID), stats); err != nil {
				return err
			}
			chunkStart = chunkEnd
		}
		return nil

	case "daily_spo2":
		items, err := s.client.GetDailySpO2(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		return s.insertMetrics(ctx, MapDailySpO2(items, userID), stats)

	case "daily_stress":
		items, err := s.client.GetDailyStress(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		return s.insertMetrics(ctx, MapDailyStress(items, userID), stats)

	case "daily_resilience":
		items, err := s.client.GetDailyResilience(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		return s.insertMetrics(ctx, MapDailyResilience(items, userID), stats)

	case "daily_cardiovascular_age":
		items, err := s.client.GetDailyCardiovascularAge(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		return s.insertMetrics(ctx, MapDailyCardiovascularAge(items, userID), stats)

	case "vo2_max":
		items, err := s.client.GetVO2Max(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		return s.insertMetrics(ctx, MapVO2Max(items, userID), stats)

	case "workout":
		items, err := s.client.GetWorkouts(ctx, token, startDate, endDate)
		if err != nil {
			return err
		}
		workouts := MapWorkouts(items, userID)
		stats.workoutsReceived += len(workouts)
		for _, w := range workouts {
			if _, err := s.db.InsertWorkout(ctx, w); err != nil {
				s.log.Warn("inserting oura workout", "error", err)
			} else {
				stats.workoutsInserted++
			}
		}
		return nil

	default:
		return fmt.Errorf("unknown data type: %s", dataType)
	}
}

// insertSleepAnalysis creates sleep_analysis health metrics from Oura long_sleep
// sessions so the dashboard chart has data. Uses noon UTC for stable dedup.
func (s *Syncer) insertSleepAnalysis(ctx context.Context, sessions []models.SleepSessionRow, stats *syncStats) error {
	var rows []models.HealthMetricRow
	for _, session := range sessions {
		qty := session.TotalSleep
		rows = append(rows, models.HealthMetricRow{
			Time:       session.Date.Add(12 * time.Hour),
			UserID:     session.UserID,
			MetricName: "sleep_analysis",
			Source:     ouraSource,
			Units:      "hr",
			Qty:        &qty,
		})
	}
	return s.insertMetrics(ctx, rows, stats)
}

// insertSleepMetrics extracts overlapping health metrics from detailed sleep data
// (resting HR, HRV, respiratory rate).
func (s *Syncer) insertSleepMetrics(ctx context.Context, items []SleepItem, userID int, stats *syncStats) error {
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
			// Oura average_breath is breaths/minute despite the OpenAPI spec
			// saying breaths/second (verified from actual API data).
			rows = append(rows, metricRow(t, userID, "respiratory_rate", "breaths/min", item.AverageBreath))
		}
	}
	return s.insertMetrics(ctx, rows, stats)
}
