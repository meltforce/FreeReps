package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/claude/freereps/internal/models"
)

// InsertAudiogram inserts a single audiogram. Returns true if inserted.
// Uses ON CONFLICT DO NOTHING on UUID PK.
func (db *DB) InsertAudiogram(ctx context.Context, row models.AudiogramRow) (bool, error) {
	tag, err := db.Pool.Exec(ctx,
		`INSERT INTO audiograms (id, user_id, sensitivity_points, start_date, source)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT DO NOTHING`,
		row.ID, row.UserID, row.SensitivityPoints, row.StartDate, row.Source)
	if err != nil {
		return false, fmt.Errorf("inserting audiogram: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// QueryAudiograms retrieves audiograms in a time range for a user.
func (db *DB) QueryAudiograms(ctx context.Context, start, end time.Time, userID int) ([]models.AudiogramRow, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, user_id, sensitivity_points, start_date, source
		 FROM audiograms
		 WHERE start_date >= $1 AND start_date < $2 AND user_id = $3
		 ORDER BY start_date DESC`,
		start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying audiograms: %w", err)
	}
	defer rows.Close()

	var result []models.AudiogramRow
	for rows.Next() {
		var r models.AudiogramRow
		if err := rows.Scan(&r.ID, &r.UserID, &r.SensitivityPoints, &r.StartDate, &r.Source); err != nil {
			return nil, fmt.Errorf("scanning audiogram: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
