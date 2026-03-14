package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/claude/freereps/internal/models"
)

// InsertECGRecording inserts a single ECG recording. Returns true if inserted.
// Uses ON CONFLICT DO NOTHING on UUID PK.
func (db *DB) InsertECGRecording(ctx context.Context, row models.ECGRecordingRow) (bool, error) {
	tag, err := db.Pool.Exec(ctx,
		`INSERT INTO ecg_recordings (id, user_id, classification, average_heart_rate, sampling_frequency, voltage_measurements, start_date, source)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 ON CONFLICT DO NOTHING`,
		row.ID, row.UserID, row.Classification, row.AverageHeartRate, row.SamplingFrequency,
		row.VoltageMeasurements, row.StartDate, row.Source)
	if err != nil {
		return false, fmt.Errorf("inserting ECG recording: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// QueryECGRecordings retrieves ECG recordings in a time range for a user.
// Returns rows ordered by start_date DESC.
func (db *DB) QueryECGRecordings(ctx context.Context, start, end time.Time, userID int) ([]models.ECGRecordingRow, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, user_id, classification, average_heart_rate, sampling_frequency, voltage_measurements, start_date, source
		 FROM ecg_recordings
		 WHERE start_date >= $1 AND start_date < $2 AND user_id = $3
		 ORDER BY start_date DESC`,
		start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying ECG recordings: %w", err)
	}
	defer rows.Close()

	var result []models.ECGRecordingRow
	for rows.Next() {
		var r models.ECGRecordingRow
		if err := rows.Scan(&r.ID, &r.UserID, &r.Classification, &r.AverageHeartRate,
			&r.SamplingFrequency, &r.VoltageMeasurements, &r.StartDate, &r.Source); err != nil {
			return nil, fmt.Errorf("scanning ECG recording: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
