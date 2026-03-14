package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/claude/freereps/internal/models"
)

// InsertVisionPrescription inserts a single vision prescription. Returns true if inserted.
// Uses ON CONFLICT DO NOTHING on UUID PK.
func (db *DB) InsertVisionPrescription(ctx context.Context, row models.VisionPrescriptionRow) (bool, error) {
	tag, err := db.Pool.Exec(ctx,
		`INSERT INTO vision_prescriptions (id, user_id, date_issued, expiration_date, prescription_type, right_eye, left_eye, source)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 ON CONFLICT DO NOTHING`,
		row.ID, row.UserID, row.DateIssued, row.ExpirationDate, row.PrescriptionType,
		row.RightEye, row.LeftEye, row.Source)
	if err != nil {
		return false, fmt.Errorf("inserting vision prescription: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// QueryVisionPrescriptions retrieves vision prescriptions in a time range for a user.
func (db *DB) QueryVisionPrescriptions(ctx context.Context, start, end time.Time, userID int) ([]models.VisionPrescriptionRow, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, user_id, date_issued, expiration_date, prescription_type, right_eye, left_eye, source
		 FROM vision_prescriptions
		 WHERE date_issued >= $1 AND date_issued < $2 AND user_id = $3
		 ORDER BY date_issued DESC`,
		start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying vision prescriptions: %w", err)
	}
	defer rows.Close()

	var result []models.VisionPrescriptionRow
	for rows.Next() {
		var r models.VisionPrescriptionRow
		if err := rows.Scan(&r.ID, &r.UserID, &r.DateIssued, &r.ExpirationDate,
			&r.PrescriptionType, &r.RightEye, &r.LeftEye, &r.Source); err != nil {
			return nil, fmt.Errorf("scanning vision prescription: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
