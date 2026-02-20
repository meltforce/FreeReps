package storage

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/claude/freereps/internal/models"
)

// InsertSleepSession upserts a sleep session (one per date per user).
func (db *DB) InsertSleepSession(ctx context.Context, row models.SleepSessionRow) error {
	_, err := db.Pool.Exec(ctx,
		`INSERT INTO sleep_sessions (user_id, date, total_sleep, asleep, core, deep, rem, in_bed, sleep_start, sleep_end, in_bed_start, in_bed_end)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		 ON CONFLICT (user_id, date) DO NOTHING`,
		row.UserID, row.Date, row.TotalSleep, row.Asleep, row.Core, row.Deep, row.REM,
		row.InBed, row.SleepStart, row.SleepEnd, row.InBedStart, row.InBedEnd)
	if err != nil {
		return fmt.Errorf("inserting sleep session: %w", err)
	}
	return nil
}

// InsertSleepStages batch-inserts sleep stage rows. Returns count inserted.
func (db *DB) InsertSleepStages(ctx context.Context, rows []models.SleepStageRow) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	query := `INSERT INTO sleep_stages (start_time, end_time, user_id, stage, duration_hr, source) VALUES `
	args := make([]any, 0, len(rows)*6)
	valueStrings := make([]string, 0, len(rows))

	for i, r := range rows {
		base := i * 6
		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6,
		))
		args = append(args, r.StartTime, r.EndTime, r.UserID, r.Stage, r.DurationHr, r.Source)
	}

	query += strings.Join(valueStrings, ",") + " ON CONFLICT DO NOTHING"

	tag, err := db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("inserting sleep stages: %w", err)
	}
	return tag.RowsAffected(), nil
}

// SleepSessionResult is a sleep session with optional stage data.
type SleepSessionResult struct {
	models.SleepSessionRow
	ID int64
}

// QuerySleepSessions retrieves sleep sessions in a date range.
func (db *DB) QuerySleepSessions(ctx context.Context, start, end time.Time, userID int) ([]SleepSessionResult, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, user_id, date, total_sleep, asleep, core, deep, rem, in_bed, sleep_start, sleep_end, in_bed_start, in_bed_end
		 FROM sleep_sessions
		 WHERE date >= $1 AND date < $2 AND user_id = $3
		 ORDER BY date DESC`,
		start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying sleep sessions: %w", err)
	}
	defer rows.Close()

	var result []SleepSessionResult
	for rows.Next() {
		var r SleepSessionResult
		if err := rows.Scan(&r.ID, &r.UserID, &r.Date, &r.TotalSleep, &r.Asleep,
			&r.Core, &r.Deep, &r.REM, &r.InBed, &r.SleepStart, &r.SleepEnd,
			&r.InBedStart, &r.InBedEnd); err != nil {
			return nil, fmt.Errorf("scanning sleep session: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// QuerySleepStages retrieves individual sleep stages in a time range.
func (db *DB) QuerySleepStages(ctx context.Context, start, end time.Time, userID int) ([]models.SleepStageRow, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT start_time, end_time, user_id, stage, duration_hr, source
		 FROM sleep_stages
		 WHERE start_time >= $1 AND start_time < $2 AND user_id = $3
		 ORDER BY start_time ASC`,
		start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying sleep stages: %w", err)
	}
	defer rows.Close()

	var result []models.SleepStageRow
	for rows.Next() {
		var r models.SleepStageRow
		if err := rows.Scan(&r.StartTime, &r.EndTime, &r.UserID, &r.Stage, &r.DurationHr, &r.Source); err != nil {
			return nil, fmt.Errorf("scanning sleep stage: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// BackfillSleepSessions synthesizes sleep sessions from existing sleep stages
// that don't yet have corresponding sessions. Called at server startup.
func (db *DB) BackfillSleepSessions(ctx context.Context, log *slog.Logger) error {
	// Get all stages
	stages, err := db.QuerySleepStages(ctx,
		time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC),
		1)
	if err != nil {
		return fmt.Errorf("querying stages for backfill: %w", err)
	}
	if len(stages) == 0 {
		return nil
	}

	// Sort by start time
	sort.Slice(stages, func(i, j int) bool {
		return stages[i].StartTime.Before(stages[j].StartTime)
	})

	// Group into nights (gap > 12h = new night)
	var nights [][]models.SleepStageRow
	var currentNight []models.SleepStageRow

	for _, stage := range stages {
		if len(currentNight) == 0 {
			currentNight = append(currentNight, stage)
			continue
		}
		lastEnd := currentNight[len(currentNight)-1].EndTime
		if stage.StartTime.Sub(lastEnd) > 12*time.Hour {
			nights = append(nights, currentNight)
			currentNight = []models.SleepStageRow{stage}
		} else {
			currentNight = append(currentNight, stage)
		}
	}
	if len(currentNight) > 0 {
		nights = append(nights, currentNight)
	}

	var created int
	for _, night := range nights {
		sleepStart := night[0].StartTime
		sleepEnd := night[len(night)-1].EndTime

		var deep, core, rem float64
		for _, s := range night {
			switch s.Stage {
			case "Deep":
				deep += s.DurationHr
			case "Core":
				core += s.DurationHr
			case "REM":
				rem += s.DurationHr
			}
		}

		totalSleep := deep + core + rem
		inBed := sleepEnd.Sub(sleepStart).Hours()
		date := sleepEnd.Truncate(24 * time.Hour)

		session := models.SleepSessionRow{
			UserID:     1,
			Date:       date,
			TotalSleep: totalSleep,
			Asleep:     totalSleep,
			Core:       core,
			Deep:       deep,
			REM:        rem,
			InBed:      inBed,
			SleepStart: sleepStart,
			SleepEnd:   sleepEnd,
			InBedStart: sleepStart,
			InBedEnd:   sleepEnd,
		}

		// ON CONFLICT DO NOTHING — won't overwrite existing sessions
		if err := db.InsertSleepSession(ctx, session); err != nil {
			return fmt.Errorf("inserting backfill session: %w", err)
		}
		created++

		// Also insert sleep_analysis health metric
		qty := totalSleep
		sleepMetric := models.HealthMetricRow{
			Time:       sleepEnd,
			UserID:     1,
			MetricName: "sleep_analysis",
			Source:     "FreeReps Backfill",
			Units:      "hr",
			Qty:        &qty,
		}
		if _, err := db.InsertHealthMetrics(ctx, []models.HealthMetricRow{sleepMetric}); err != nil {
			return fmt.Errorf("inserting backfill sleep_analysis metric: %w", err)
		}
	}

	log.Info("sleep session backfill complete", "nights", len(nights), "sessions_created", created)
	return nil
}
