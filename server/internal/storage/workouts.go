package storage

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/claude/freereps/internal/models"
	"github.com/google/uuid"
)

// alphaWorkoutNamespace is the UUID namespace for deterministic synthetic Alpha workout IDs.
var alphaWorkoutNamespace = uuid.MustParse("7ba7b810-9dad-11d1-80b4-00c04fd430c8")

// InsertWorkout inserts a workout row. Returns true if inserted, false if duplicate.
func (db *DB) InsertWorkout(ctx context.Context, row models.WorkoutRow) (bool, error) {
	tag, err := db.Pool.Exec(ctx,
		`INSERT INTO workouts (id, user_id, name, source, start_time, end_time, duration_sec, location, is_indoor,
		 active_energy_burned, active_energy_units, total_energy, total_energy_units,
		 distance, distance_units, avg_heart_rate, max_heart_rate, min_heart_rate,
		 elevation_up, elevation_down, raw_json)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
		 ON CONFLICT DO NOTHING`,
		row.ID, row.UserID, row.Name, row.Source, row.StartTime, row.EndTime, row.DurationSec,
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

	// 10 params per row; PostgreSQL extended protocol limited to 65535 params.
	const batchSize = 6000
	var total int64

	for start := 0; start < len(rows); start += batchSize {
		end := start + batchSize
		if end > len(rows) {
			end = len(rows)
		}
		batch := rows[start:end]

		query := `INSERT INTO workout_routes (time, workout_id, user_id, latitude, longitude, altitude, speed, course, horizontal_accuracy, vertical_accuracy) VALUES `
		args := make([]any, 0, len(batch)*10)
		valueStrings := make([]string, 0, len(batch))

		for i, r := range batch {
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
			return total, fmt.Errorf("inserting workout routes: %w", err)
		}
		total += tag.RowsAffected()
	}
	return total, nil
}

// WorkoutDetail is a workout with its HR and route data.
type WorkoutDetail struct {
	models.WorkoutRow
	HeartRateData []models.WorkoutHRRow
	RouteData     []models.WorkoutRouteRow
}

// QueryWorkouts retrieves workouts in a time range, optionally filtered by type name.
// Deduplicates overlapping workouts from different sources using source priority:
// when two workouts start within the same 5-minute window, only the highest-priority
// source's workout is returned. Excludes raw_json to keep the list payload small.
func (db *DB) QueryWorkouts(ctx context.Context, start, end time.Time, userID int, nameFilter string) ([]models.WorkoutRow, error) {
	priorities := db.ResolveSourcePriority(ctx, userID, "activity")
	priorityExpr := sourcePriorityCaseSQL(priorities)
	where := `start_time >= $1 AND start_time < $2 AND user_id = $3`
	args := []any{start, end, userID}
	if nameFilter != "" {
		where += ` AND name = $4`
		args = append(args, nameFilter)
	}
	query := fmt.Sprintf(
		`WITH ranked AS (
			SELECT *, ROW_NUMBER() OVER (
				PARTITION BY date_trunc('hour', start_time) + INTERVAL '5 min' * FLOOR(EXTRACT(MINUTE FROM start_time) / 5)
				ORDER BY %s
			) AS rn
			FROM workouts
			WHERE %s
		)
		SELECT id, user_id, name, source, start_time, end_time, duration_sec, location, is_indoor,
			active_energy_burned, active_energy_units, total_energy, total_energy_units,
			distance, distance_units, avg_heart_rate, max_heart_rate, min_heart_rate,
			elevation_up, elevation_down
		FROM ranked WHERE rn = 1
		ORDER BY start_time DESC`, priorityExpr, where)
	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying workouts: %w", err)
	}
	defer rows.Close()

	return scanWorkoutListRows(rows)
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

// scanWorkoutListRows scans workout rows without raw_json (for list queries).
func scanWorkoutListRows(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]models.WorkoutRow, error) {
	var result []models.WorkoutRow
	for rows.Next() {
		var w models.WorkoutRow
		if err := rows.Scan(&w.ID, &w.UserID, &w.Name, &w.Source, &w.StartTime, &w.EndTime, &w.DurationSec,
			&w.Location, &w.IsIndoor,
			&w.ActiveEnergyBurned, &w.ActiveEnergyUnits, &w.TotalEnergy, &w.TotalEnergyUnits,
			&w.Distance, &w.DistanceUnits, &w.AvgHeartRate, &w.MaxHeartRate, &w.MinHeartRate,
			&w.ElevationUp, &w.ElevationDown); err != nil {
			return nil, fmt.Errorf("scanning workout: %w", err)
		}
		result = append(result, w)
	}
	return result, rows.Err()
}

// QueryWorkoutsMerged returns workouts enriched with Alpha Progression session names.
// Apple/Oura workouts near an Alpha session get the session name for display.
// Alpha sessions with no nearby workout get a synthetic workout entry.
func (db *DB) QueryWorkoutsMerged(ctx context.Context, start, end time.Time, userID int, nameFilter string) ([]models.WorkoutRow, error) {
	workouts, err := db.QueryWorkouts(ctx, start, end, userID, nameFilter)
	if err != nil {
		return nil, err
	}

	// Fetch Alpha sessions with 2h padding to catch sessions just outside the range.
	alphaSessions, err := db.QueryAlphaSessions(ctx, start.Add(-2*time.Hour), end.Add(2*time.Hour), userID)
	if err != nil {
		return nil, err
	}
	if len(alphaSessions) == 0 {
		return workouts, nil
	}

	// Match Alpha sessions to workouts by nearest time within ±2h.
	matched := make(map[int]bool)    // index into workouts
	alphaUsed := make(map[int]bool)  // index into alphaSessions

	type pair struct {
		wi, ai int
		dist   time.Duration
	}
	var pairs []pair
	for wi, w := range workouts {
		for ai, a := range alphaSessions {
			dist := w.StartTime.Sub(a.SessionDate)
			if dist < 0 {
				dist = -dist
			}
			if dist <= 2*time.Hour {
				pairs = append(pairs, pair{wi, ai, dist})
			}
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].dist < pairs[j].dist })

	for _, p := range pairs {
		if matched[p.wi] || alphaUsed[p.ai] {
			continue
		}
		workouts[p.wi].AlphaSessionName = alphaSessions[p.ai].SessionName
		matched[p.wi] = true
		alphaUsed[p.ai] = true
	}

	// Create synthetic workouts for unmatched Alpha sessions.
	for ai, a := range alphaSessions {
		if alphaUsed[ai] {
			continue
		}
		// Skip if outside the requested range.
		if a.SessionDate.Before(start) || !a.SessionDate.Before(end) {
			continue
		}
		// Skip if name filter is set and doesn't match the synthetic base name.
		if nameFilter != "" && nameFilter != "Traditional Strength Training" {
			continue
		}
		dur := parseAlphaDuration(a.SessionDuration)
		workouts = append(workouts, models.WorkoutRow{
			ID:               uuid.NewSHA1(alphaWorkoutNamespace, []byte("alpha:"+a.SessionDate.Format(time.RFC3339)+":"+a.SessionName)),
			UserID:           userID,
			Name:             "Traditional Strength Training",
			Source:           "Alpha Progression",
			StartTime:        a.SessionDate,
			EndTime:          a.SessionDate.Add(dur),
			DurationSec:      dur.Seconds(),
			AlphaSessionName: a.SessionName,
		})
	}

	sort.Slice(workouts, func(i, j int) bool {
		return workouts[i].StartTime.After(workouts[j].StartTime)
	})
	return workouts, nil
}

// parseAlphaDuration parses Alpha Progression duration strings like "1:02 hr".
func parseAlphaDuration(s string) time.Duration {
	s = strings.TrimSpace(strings.TrimSuffix(s, "hr"))
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0
	}
	hours, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	mins, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	return time.Duration(hours)*time.Hour + time.Duration(mins)*time.Minute
}
