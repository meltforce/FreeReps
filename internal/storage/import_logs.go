package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ImportLog represents a single import operation's outcome.
type ImportLog struct {
	ID               int64            `json:"id"`
	UserID           int              `json:"user_id"`
	CreatedAt        time.Time        `json:"created_at"`
	Source           string           `json:"source"`
	Status           string           `json:"status"`
	MetricsReceived  int              `json:"metrics_received"`
	MetricsInserted  int64            `json:"metrics_inserted"`
	WorkoutsReceived int              `json:"workouts_received"`
	WorkoutsInserted int              `json:"workouts_inserted"`
	SleepSessions    int              `json:"sleep_sessions"`
	SetsInserted     int64            `json:"sets_inserted"`
	DurationMs       *int             `json:"duration_ms"`
	ErrorMessage     *string          `json:"error_message"`
	Metadata         *json.RawMessage `json:"metadata"`
}

// InsertImportLog creates a new import log entry and returns its ID.
func (db *DB) InsertImportLog(ctx context.Context, log ImportLog) (int64, error) {
	var id int64
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO import_logs (user_id, source, status, metrics_received, metrics_inserted,
		 workouts_received, workouts_inserted, sleep_sessions, sets_inserted, duration_ms, error_message, metadata)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		 RETURNING id`,
		log.UserID, log.Source, log.Status, log.MetricsReceived, log.MetricsInserted,
		log.WorkoutsReceived, log.WorkoutsInserted, log.SleepSessions, log.SetsInserted,
		log.DurationMs, log.ErrorMessage, log.Metadata,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("inserting import log: %w", err)
	}
	return id, nil
}

// UpdateImportLog updates an existing import log entry (typically from "running" to "success" or "error").
func (db *DB) UpdateImportLog(ctx context.Context, id int64, log ImportLog) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE import_logs SET
		 status = $2, metrics_received = $3, metrics_inserted = $4,
		 workouts_received = $5, workouts_inserted = $6, sleep_sessions = $7,
		 sets_inserted = $8, duration_ms = $9, error_message = $10, metadata = $11
		 WHERE id = $1`,
		id, log.Status, log.MetricsReceived, log.MetricsInserted,
		log.WorkoutsReceived, log.WorkoutsInserted, log.SleepSessions,
		log.SetsInserted, log.DurationMs, log.ErrorMessage, log.Metadata,
	)
	if err != nil {
		return fmt.Errorf("updating import log %d: %w", id, err)
	}
	return nil
}

// QueryImportLogs returns the most recent import logs for a user.
func (db *DB) QueryImportLogs(ctx context.Context, userID, limit int) ([]ImportLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.Pool.Query(ctx,
		`SELECT id, user_id, created_at, source, status, metrics_received, metrics_inserted,
		 workouts_received, workouts_inserted, sleep_sessions, sets_inserted, duration_ms, error_message, metadata
		 FROM import_logs
		 WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`,
		userID, limit)
	if err != nil {
		return nil, fmt.Errorf("querying import logs: %w", err)
	}
	defer rows.Close()

	var result []ImportLog
	for rows.Next() {
		var l ImportLog
		if err := rows.Scan(&l.ID, &l.UserID, &l.CreatedAt, &l.Source, &l.Status,
			&l.MetricsReceived, &l.MetricsInserted, &l.WorkoutsReceived, &l.WorkoutsInserted,
			&l.SleepSessions, &l.SetsInserted, &l.DurationMs, &l.ErrorMessage, &l.Metadata); err != nil {
			return nil, fmt.Errorf("scanning import log: %w", err)
		}
		result = append(result, l)
	}
	return result, rows.Err()
}
