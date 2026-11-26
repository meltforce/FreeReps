package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// IngestCheckpoint is where one client's delivered data of one domain ends.
type IngestCheckpoint struct {
	Domain       string    `json:"domain"`
	Item         string    `json:"item,omitempty"`
	NewestSample time.Time `json:"newest_sample"`
	LastImportAt time.Time `json:"last_import_at"`
	// RowCount is the number of stored rows; absent while not counted.
	RowCount *int64 `json:"row_count,omitempty"`
}

// AdvanceIngestCheckpoints records, per item of one domain, the newest sample
// time of a batch the caller has just stored without error, and adds the rows
// that batch created to the item's row count.
//
// newest_sample only moves forward. A client re-sending an older window — the
// iOS app re-sends the last 24 hours daily, and the backfill resumes at a
// cursor — must not pull the checkpoint back behind data the server holds.
//
// A new checkpoint starts with no row count: rows of that item may predate it.
func (db *DB) AdvanceIngestCheckpoints(ctx context.Context, userID int, client, domain string, newest map[string]time.Time, added map[string]int64) error {
	if len(newest) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for item, t := range newest {
		batch.Queue(
			`INSERT INTO ingest_checkpoints (user_id, client, domain, item, newest_sample, last_import_at)
			 VALUES ($1, $2, $3, $4, $5, NOW())
			 ON CONFLICT (user_id, client, domain, item) DO UPDATE
			    SET newest_sample  = GREATEST(ingest_checkpoints.newest_sample, EXCLUDED.newest_sample),
			        last_import_at = NOW(),
			        row_count      = ingest_checkpoints.row_count + $6`,
			userID, client, domain, item, t, added[item])
	}
	if err := db.Pool.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("advancing %s checkpoints: %w", domain, err)
	}
	return nil
}

// IngestCheckpoints returns every checkpoint of one client, ordered by domain
// and item.
func (db *DB) IngestCheckpoints(ctx context.Context, userID int, client string) ([]IngestCheckpoint, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT domain, item, newest_sample, last_import_at, row_count
		   FROM ingest_checkpoints
		  WHERE user_id = $1 AND client = $2
		  ORDER BY domain, item`,
		userID, client)
	if err != nil {
		return nil, fmt.Errorf("querying ingest checkpoints: %w", err)
	}
	defer rows.Close()

	out := []IngestCheckpoint{}
	for rows.Next() {
		var c IngestCheckpoint
		if err := rows.Scan(&c.Domain, &c.Item, &c.NewestSample, &c.LastImportAt, &c.RowCount); err != nil {
			return nil, fmt.Errorf("scanning ingest checkpoint: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// recountWindow bounds one counting query of RecountIngestCheckpoints. A count
// over every chunk of health_metrics in one statement exhausted the memory of
// the 2 GB host (archive/INCIDENTS.md, 2026-09-27).
const recountWindow = 90 * 24 * time.Hour

// RecountIngestCheckpoints sets the row count of every metrics checkpoint that
// has none, from health_metrics.
//
// An ingest that stores rows while this runs is counted or not depending on
// whether its window was already read, so the result can be off by that batch.
func (db *DB) RecountIngestCheckpoints(ctx context.Context) error {
	type key struct {
		userID int
		client string
	}
	pending := map[key][]string{}
	rows, err := db.Pool.Query(ctx,
		`SELECT user_id, client, item FROM ingest_checkpoints
		  WHERE domain = 'metrics' AND row_count IS NULL`)
	if err != nil {
		return fmt.Errorf("listing uncounted checkpoints: %w", err)
	}
	for rows.Next() {
		var k key
		var item string
		if err := rows.Scan(&k.userID, &k.client, &item); err != nil {
			rows.Close()
			return fmt.Errorf("scanning uncounted checkpoint: %w", err)
		}
		pending[k] = append(pending[k], item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("listing uncounted checkpoints: %w", err)
	}
	if len(pending) == 0 {
		return nil
	}

	var oldest *time.Time
	if err := db.Pool.QueryRow(ctx, `SELECT min(time) FROM health_metrics`).Scan(&oldest); err != nil {
		return fmt.Errorf("finding the oldest metric: %w", err)
	}

	for k, items := range pending {
		counts := map[string]int64{}
		for _, item := range items {
			counts[item] = 0
		}
		if oldest != nil {
			for from := *oldest; ; from = from.Add(recountWindow) {
				to := from.Add(recountWindow)
				last := to.After(time.Now())
				// The last window has no upper bound: a sample may be dated ahead.
				q := `SELECT metric_name, count(*) FROM health_metrics
				       WHERE user_id = $1 AND client = $2 AND metric_name = ANY($3) AND time >= $4`
				args := []any{k.userID, k.client, items, from}
				if !last {
					q += ` AND time < $5`
					args = append(args, to)
				}
				if err := db.addCounts(ctx, counts, q+` GROUP BY metric_name`, args...); err != nil {
					return err
				}
				if last {
					break
				}
			}
		}
		batch := &pgx.Batch{}
		for item, n := range counts {
			batch.Queue(
				`UPDATE ingest_checkpoints SET row_count = $4
				  WHERE user_id = $1 AND client = $2 AND domain = 'metrics' AND item = $3
				    AND row_count IS NULL`,
				k.userID, k.client, item, n)
		}
		if err := db.Pool.SendBatch(ctx, batch).Close(); err != nil {
			return fmt.Errorf("storing row counts: %w", err)
		}
	}
	return nil
}

func (db *DB) addCounts(ctx context.Context, counts map[string]int64, query string, args ...any) error {
	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("counting metrics: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var n int64
		if err := rows.Scan(&name, &n); err != nil {
			return fmt.Errorf("counting metrics: %w", err)
		}
		counts[name] += n
	}
	return rows.Err()
}

// StoredRowCounts returns how many rows the user has in the tables behind the
// checkpoint domains other than metrics, keyed by domain. These tables carry no
// client, so the count covers every source. workout_routes counts workouts
// that have a route; the time bound lets each probe skip the other chunks.
func (db *DB) StoredRowCounts(ctx context.Context, userID int) (map[string]int64, error) {
	out := map[string]int64{}
	for domain, q := range map[string]string{
		"workouts":           `SELECT count(*) FROM workouts WHERE user_id = $1`,
		"category_samples":   `SELECT count(*) FROM category_samples WHERE user_id = $1`,
		"activity_summaries": `SELECT count(*) FROM activity_summaries WHERE user_id = $1`,
		"state_of_mind":      `SELECT count(*) FROM state_of_mind WHERE user_id = $1`,
		"workout_routes": `SELECT count(*) FROM workouts w WHERE w.user_id = $1 AND EXISTS (
		                     SELECT 1 FROM workout_routes r
		                      WHERE r.user_id = w.user_id AND r.workout_id = w.id
		                        AND r.time >= w.start_time - interval '1 hour'
		                        AND r.time <= w.end_time + interval '1 hour')`,
	} {
		var n int64
		if err := db.Pool.QueryRow(ctx, q, userID).Scan(&n); err != nil {
			return nil, fmt.Errorf("counting %s: %w", domain, err)
		}
		out[domain] = n
	}
	return out, nil
}
