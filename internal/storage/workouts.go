package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/claude/freereps/internal/models"
	"github.com/google/uuid"
)

// InsertWorkout inserts a workout row. Returns true if inserted, false if duplicate.
func (db *DB) InsertWorkout(ctx context.Context, row models.WorkoutRow) (bool, error) {
	tag, err := db.Pool.Exec(ctx,
		`INSERT INTO workouts (id, user_id, name, start_time, end_time, duration_sec, location, is_indoor,
		 active_energy_burned, active_energy_units, total_energy, total_energy_units,
		 distance, distance_units, avg_heart_rate, max_heart_rate, min_heart_rate,
		 elevation_up, elevation_down, raw_json)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
		 ON CONFLICT DO NOTHING`,
		row.ID, row.UserID, row.Name, row.StartTime, row.EndTime, row.DurationSec,
		row.Location, row.IsIndoor,
		row.ActiveEnergyBurned, row.ActiveEnergyUnits, row.TotalEnergy, row.TotalEnergyUnits,
		row.Distance, row.DistanceUnits, row.AvgHeartRate, row.MaxHeartRate, row.MinHeartRate,
		row.ElevationUp, row.ElevationDown, row.RawJSON)
	if err != nil {
		return false, fmt.Errorf("inserting workout: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// InsertWorkoutHeartRate batch-inserts workout HR data points. Returns count inserted.
func (db *DB) InsertWorkoutHeartRate(ctx context.Context, rows []models.WorkoutHRRow) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	query := `INSERT INTO workout_heart_rate (time, workout_id, user_id, min_bpm, avg_bpm, max_bpm, source) VALUES `
	args := make([]any, 0, len(rows)*7)
	valueStrings := make([]string, 0, len(rows))

	for i, r := range rows {
		base := i * 7
		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7,
		))
		args = append(args, r.Time, r.WorkoutID, r.UserID, r.MinBPM, r.AvgBPM, r.MaxBPM, r.Source)
	}

	query += strings.Join(valueStrings, ",") + " ON CONFLICT DO NOTHING"

	tag, err := db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("inserting workout heart rate: %w", err)
	}
	return tag.RowsAffected(), nil
}

// InsertWorkoutRoutes batch-inserts workout route points. Returns count inserted.
func (db *DB) InsertWorkoutRoutes(ctx context.Context, rows []models.WorkoutRouteRow) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	query := `INSERT INTO workout_routes (time, workout_id, user_id, latitude, longitude, altitude, speed, course, horizontal_accuracy, vertical_accuracy) VALUES `
	args := make([]any, 0, len(rows)*10)
	valueStrings := make([]string, 0, len(rows))

	for i, r := range rows {
		base := i * 10
		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9, base+10,
		))
		args = append(args, r.Time, r.WorkoutID, r.UserID, r.Latitude, r.Longitude,
			r.Altitude, r.Speed, r.Course, r.HorizontalAccuracy, r.VerticalAccuracy)
	}

	query += strings.Join(valueStrings, ",") + " ON CONFLICT DO NOTHING"

	tag, err := db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("inserting workout routes: %w", err)
	}
	return tag.RowsAffected(), nil
}

// WorkoutDetail is a workout with its HR and route data.
type WorkoutDetail struct {
	models.WorkoutRow
	HeartRateData []models.WorkoutHRRow
	RouteData     []models.WorkoutRouteRow
}

// QueryWorkouts retrieves workouts in a time range.
func (db *DB) QueryWorkouts(ctx context.Context, start, end time.Time, userID int) ([]models.WorkoutRow, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, user_id, name, start_time, end_time, duration_sec, location, is_indoor,
		 active_energy_burned, active_energy_units, total_energy, total_energy_units,
		 distance, distance_units, avg_heart_rate, max_heart_rate, min_heart_rate,
		 elevation_up, elevation_down, raw_json
		 FROM workouts
		 WHERE start_time >= $1 AND start_time < $2 AND user_id = $3
		 ORDER BY start_time DESC`,
		start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying workouts: %w", err)
	}
	defer rows.Close()

	return scanWorkoutRows(rows)
}

// GetWorkout retrieves a single workout by ID with all associated data.
func (db *DB) GetWorkout(ctx context.Context, workoutID uuid.UUID, userID int) (*WorkoutDetail, error) {
	row := db.Pool.QueryRow(ctx,
		`SELECT id, user_id, name, start_time, end_time, duration_sec, location, is_indoor,
		 active_energy_burned, active_energy_units, total_energy, total_energy_units,
		 distance, distance_units, avg_heart_rate, max_heart_rate, min_heart_rate,
		 elevation_up, elevation_down, raw_json
		 FROM workouts
		 WHERE id = $1 AND user_id = $2`,
		workoutID, userID)

	var w models.WorkoutRow
	err := row.Scan(&w.ID, &w.UserID, &w.Name, &w.StartTime, &w.EndTime, &w.DurationSec,
		&w.Location, &w.IsIndoor,
		&w.ActiveEnergyBurned, &w.ActiveEnergyUnits, &w.TotalEnergy, &w.TotalEnergyUnits,
		&w.Distance, &w.DistanceUnits, &w.AvgHeartRate, &w.MaxHeartRate, &w.MinHeartRate,
		&w.ElevationUp, &w.ElevationDown, &w.RawJSON)
	if err != nil {
		return nil, fmt.Errorf("querying workout: %w", err)
	}

	detail := &WorkoutDetail{WorkoutRow: w}

	// Get HR data
	hrRows, err := db.Pool.Query(ctx,
		`SELECT time, workout_id, user_id, min_bpm, avg_bpm, max_bpm, source
		 FROM workout_heart_rate
		 WHERE workout_id = $1 AND user_id = $2
		 ORDER BY time ASC`,
		workoutID, userID)
	if err != nil {
		return nil, fmt.Errorf("querying workout HR: %w", err)
	}
	defer hrRows.Close()

	for hrRows.Next() {
		var hr models.WorkoutHRRow
		if err := hrRows.Scan(&hr.Time, &hr.WorkoutID, &hr.UserID, &hr.MinBPM, &hr.AvgBPM, &hr.MaxBPM, &hr.Source); err != nil {
			return nil, fmt.Errorf("scanning workout HR: %w", err)
		}
		detail.HeartRateData = append(detail.HeartRateData, hr)
	}
	if err := hrRows.Err(); err != nil {
		return nil, err
	}

	// Get route data
	routeRows, err := db.Pool.Query(ctx,
		`SELECT time, workout_id, user_id, latitude, longitude, altitude, speed, course, horizontal_accuracy, vertical_accuracy
		 FROM workout_routes
		 WHERE workout_id = $1 AND user_id = $2
		 ORDER BY time ASC`,
		workoutID, userID)
	if err != nil {
		return nil, fmt.Errorf("querying workout routes: %w", err)
	}
	defer routeRows.Close()

	for routeRows.Next() {
		var r models.WorkoutRouteRow
		if err := routeRows.Scan(&r.Time, &r.WorkoutID, &r.UserID, &r.Latitude, &r.Longitude,
			&r.Altitude, &r.Speed, &r.Course, &r.HorizontalAccuracy, &r.VerticalAccuracy); err != nil {
			return nil, fmt.Errorf("scanning workout route: %w", err)
		}
		detail.RouteData = append(detail.RouteData, r)
	}

	return detail, routeRows.Err()
}

func scanWorkoutRows(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]models.WorkoutRow, error) {
	var result []models.WorkoutRow
	for rows.Next() {
		var w models.WorkoutRow
		if err := rows.Scan(&w.ID, &w.UserID, &w.Name, &w.StartTime, &w.EndTime, &w.DurationSec,
			&w.Location, &w.IsIndoor,
			&w.ActiveEnergyBurned, &w.ActiveEnergyUnits, &w.TotalEnergy, &w.TotalEnergyUnits,
			&w.Distance, &w.DistanceUnits, &w.AvgHeartRate, &w.MaxHeartRate, &w.MinHeartRate,
			&w.ElevationUp, &w.ElevationDown, &w.RawJSON); err != nil {
			return nil, fmt.Errorf("scanning workout: %w", err)
		}
		result = append(result, w)
	}
	return result, rows.Err()
}
