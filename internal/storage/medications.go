package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/claude/freereps/internal/models"
)

// InsertMedication inserts a single medication record. Returns true if inserted.
// Uses ON CONFLICT DO NOTHING on UUID PK.
func (db *DB) InsertMedication(ctx context.Context, row models.MedicationRow) (bool, error) {
	tag, err := db.Pool.Exec(ctx,
		`INSERT INTO medications (id, user_id, name, dosage, log_status, start_date, end_date, source)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 ON CONFLICT DO NOTHING`,
		row.ID, row.UserID, row.Name, row.Dosage, row.LogStatus,
		row.StartDate, row.EndDate, row.Source)
	if err != nil {
		return false, fmt.Errorf("inserting medication: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// QueryMedications retrieves medications in a time range for a user.
func (db *DB) QueryMedications(ctx context.Context, start, end time.Time, userID int) ([]models.MedicationRow, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, user_id, name, dosage, log_status, start_date, end_date, source
		 FROM medications
		 WHERE start_date >= $1 AND start_date < $2 AND user_id = $3
		 ORDER BY start_date DESC`,
		start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying medications: %w", err)
	}
	defer rows.Close()

	var result []models.MedicationRow
	for rows.Next() {
		var r models.MedicationRow
		if err := rows.Scan(&r.ID, &r.UserID, &r.Name, &r.Dosage, &r.LogStatus,
			&r.StartDate, &r.EndDate, &r.Source); err != nil {
			return nil, fmt.Errorf("scanning medication: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
