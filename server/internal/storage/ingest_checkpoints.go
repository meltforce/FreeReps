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
}

// AdvanceIngestCheckpoints records, per item of one domain, the newest sample
// time of a batch the caller has just stored without error.
//
// newest_sample only moves forward. A client re-sending an older window — the
// iOS app re-sends the last 24 hours daily, and the backfill resumes at a
// cursor — must not pull the checkpoint back behind data the server holds.
func (db *DB) AdvanceIngestCheckpoints(ctx context.Context, userID int, client, domain string, newest map[string]time.Time) error {
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
			        last_import_at = NOW()`,
			userID, client, domain, item, t)
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
		`SELECT domain, item, newest_sample, last_import_at
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
		if err := rows.Scan(&c.Domain, &c.Item, &c.NewestSample, &c.LastImportAt); err != nil {
			return nil, fmt.Errorf("scanning ingest checkpoint: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
