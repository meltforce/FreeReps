package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/claude/freereps/internal/models"
)

// InsertStateOfMind batch-inserts state of mind rows. Returns count inserted.
// Uses ON CONFLICT DO NOTHING on UUID PK.
func (db *DB) InsertStateOfMind(ctx context.Context, rows []models.StateOfMindRow) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	query := `INSERT INTO state_of_mind (id, user_id, kind, valence, labels, associations, start_date, source) VALUES `
	args := make([]any, 0, len(rows)*8)
	valueStrings := make([]string, 0, len(rows))

	for i, r := range rows {
		base := i * 8
		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8,
		))
		args = append(args, r.ID, r.UserID, r.Kind, r.Valence, r.Labels, r.Associations,
			r.StartDate, r.Source)
	}

	query += strings.Join(valueStrings, ",") + " ON CONFLICT DO NOTHING"

	tag, err := db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("inserting state of mind: %w", err)
	}
	return tag.RowsAffected(), nil
}

// QueryStateOfMind retrieves state of mind records in a time range for a user.
func (db *DB) QueryStateOfMind(ctx context.Context, start, end time.Time, userID int) ([]models.StateOfMindRow, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, user_id, kind, valence, labels, associations, start_date, source
		 FROM state_of_mind
		 WHERE start_date >= $1 AND start_date < $2 AND user_id = $3
		 ORDER BY start_date DESC`,
		start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying state of mind: %w", err)
	}
	defer rows.Close()

	var result []models.StateOfMindRow
	for rows.Next() {
		var r models.StateOfMindRow
		if err := rows.Scan(&r.ID, &r.UserID, &r.Kind, &r.Valence, &r.Labels,
			&r.Associations, &r.StartDate, &r.Source); err != nil {
			return nil, fmt.Errorf("scanning state of mind: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
