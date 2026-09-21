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

// hevyWorkoutNamespace is the UUID namespace for deterministic synthetic Hevy workout IDs.
// Unlike Alpha, the payload is Hevy's own workout id, so the synthetic id stays
// stable even when the session is renamed or its start time is corrected.
var hevyWorkoutNamespace = uuid.MustParse("8ba7b810-9dad-11d1-80b4-00c04fd430c8")

// syntheticWorkoutName is the workout type reported for sessions that exist only
// in workout_sets. The frontend type filters match on this value.
const syntheticWorkoutName = "Traditional Strength Training"

// syntheticWorkoutID derives the deterministic id for a strength session that has
// no row in the workouts table.
func syntheticWorkoutID(s SetSessionInfo) uuid.UUID {
	if s.Source == "Hevy" && s.ExternalID != "" {
		return uuid.NewSHA1(hevyWorkoutNamespace, []byte("hevy:workout:"+s.ExternalID))
	}
	return uuid.NewSHA1(alphaWorkoutNamespace,
		[]byte("alpha:"+s.SessionDate.Format(time.RFC3339)+":"+s.SessionName))
}

// syntheticWorkoutEnd returns the session end. Hevy reports it directly; Alpha
// only carries a duration string that has to be parsed and added to the start.
func syntheticWorkoutEnd(s SetSessionInfo) time.Time {
	if s.SessionEnd != nil && !s.SessionEnd.IsZero() {
		return *s.SessionEnd
	}
	return s.SessionDate.Add(parseAlphaDuration(s.SessionDuration))
}

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

// FillWorkoutHeartRateFromMetrics derives a workout's heart rate series from the
// stored heart_rate metrics of one source and attaches it to that workout.
//
// It exists for the Oura API path. The workout object there carries no heart
// rate — its fields are id, activity, calories, day, distance, end_datetime,
// intensity, label, source and start_datetime — while
// /v2/usercollection/heartrate reports every sample with a source of its own,
// "workout" among them. For a user whose workouts reach FreeReps over the Oura
// API alone, those samples are the only heart rate the workout can have.
//
// The samples are grouped per minute, and the summary on the workout row
// averages the minutes rather than the samples. An average over samples is an
// average of the sampling rate as much as of the heart rate, which is the defect
// the dashboard aggregation was corrected for on 2026-09-20 (see DECISIONS.md).
// Measured for a 103-minute session on 2026-09-21, where a 5-second burst over
// the first six minutes holds 77 of the 92 samples: 98.32 over the samples
// against 96.16 over the 16 minutes.
//
// Idempotent, because the sync re-reads the same days: existing rows are kept
// and the summary is recomputed from what the table holds afterwards. A workout
// whose interval carries no sample of that source leaves both tables unchanged.
// Returns the number of minute rows inserted.
func (db *DB) FillWorkoutHeartRateFromMetrics(ctx context.Context, userID int, workoutID uuid.UUID, source string, start, end time.Time) (int64, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// COALESCE covers both metric shapes: Oura stores one value per sample in
	// qty, the Health Auto Export path stores min/avg/max per row.
	tag, err := tx.Exec(ctx,
		`INSERT INTO workout_heart_rate (time, workout_id, user_id, min_bpm, avg_bpm, max_bpm, source)
		 SELECT date_trunc('minute', time), $1, $2,
		        MIN(COALESCE(min_val, qty)), AVG(COALESCE(avg_val, qty)), MAX(COALESCE(max_val, qty)), $3
		 FROM health_metrics
		 WHERE user_id = $2 AND metric_name = 'heart_rate' AND source = $3
		   AND time >= $4 AND time < $5
		   AND COALESCE(avg_val, qty) IS NOT NULL
		 GROUP BY 1
		 ON CONFLICT DO NOTHING`,
		workoutID, userID, source, start, end)
	if err != nil {
		return 0, fmt.Errorf("deriving workout heart rate: %w", err)
	}
	inserted := tag.RowsAffected()

	// The summary reads the table rather than the insert above, so a workout
	// that already carries minutes from an earlier cycle keeps a summary over
	// all of them.
	if _, err := tx.Exec(ctx,
		`UPDATE workouts w SET
		   avg_heart_rate = s.avg_bpm,
		   min_heart_rate = s.min_bpm,
		   max_heart_rate = s.max_bpm
		 FROM (SELECT AVG(avg_bpm) AS avg_bpm, MIN(min_bpm) AS min_bpm, MAX(max_bpm) AS max_bpm
		       FROM workout_heart_rate
		       WHERE workout_id = $1 AND user_id = $2) s
		 WHERE w.id = $1 AND w.user_id = $2 AND s.avg_bpm IS NOT NULL`,
		workoutID, userID); err != nil {
		return 0, fmt.Errorf("updating workout heart rate summary: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing derived workout heart rate: %w", err)
	}
	return inserted, nil
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
//
// The returned row carries every source of its window in Sources. An Oura
// session arrives twice — once over the API and once as the copy the Oura app
// writes into HealthKit — and the second one wins here whenever Apple Health
// ranks first, which is what the display needs the list for: the row it shows
// was recorded by Oura, and labelling it with the hub it came through names the
// wrong provider. The list rests on the same assumption as the deduplication
// itself, that two workouts starting in one 5-minute window are one session.
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
		`WITH bucketed AS (
			SELECT *, date_trunc('hour', start_time)
			          + INTERVAL '5 min' * FLOOR(EXTRACT(MINUTE FROM start_time) / 5) AS bucket
			FROM workouts
			WHERE %s
		), ranked AS (
			SELECT *,
				ROW_NUMBER() OVER (PARTITION BY bucket ORDER BY %s) AS rn,
				array_agg(source) OVER (PARTITION BY bucket) AS bucket_sources
			FROM bucketed
		)
		SELECT id, user_id, name, source, start_time, end_time, duration_sec, location, is_indoor,
			active_energy_burned, active_energy_units, total_energy, total_energy_units,
			distance, distance_units, avg_heart_rate, max_heart_rate, min_heart_rate,
			elevation_up, elevation_down, bucket_sources
		FROM ranked WHERE rn = 1
		ORDER BY start_time DESC`, where, priorityExpr)
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
		var bucketSources []string
		if err := rows.Scan(&w.ID, &w.UserID, &w.Name, &w.Source, &w.StartTime, &w.EndTime, &w.DurationSec,
			&w.Location, &w.IsIndoor,
			&w.ActiveEnergyBurned, &w.ActiveEnergyUnits, &w.TotalEnergy, &w.TotalEnergyUnits,
			&w.Distance, &w.DistanceUnits, &w.AvgHeartRate, &w.MaxHeartRate, &w.MinHeartRate,
			&w.ElevationUp, &w.ElevationDown, &bucketSources); err != nil {
			return nil, fmt.Errorf("scanning workout: %w", err)
		}
		w.Sources = distinctSources(bucketSources)
		result = append(result, w)
	}
	return result, rows.Err()
}

// distinctSources reduces a window's source values to a sorted set. The order
// array_agg produces over a window is not defined, and the list reaches the
// browser, where a stable order keeps a rerender from reordering a label.
func distinctSources(sources []string) []string {
	if len(sources) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(sources))
	out := make([]string, 0, len(sources))
	for _, s := range sources {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// QueryWorkoutsMerged returns workouts enriched with strength session names from
// workout_sets. Apple/Oura workouts near such a session get its name for display;
// sessions with no nearby workout get a synthetic workout entry.
//
// Neither Alpha Progression nor Hevy writes rows into the workouts table — that
// was tried for Alpha and reverted in commit 411f4c1, because the same training
// session already arrives from Apple Health with heart rate data and the two
// rows cannot be deduplicated reliably by start time.
func (db *DB) QueryWorkoutsMerged(ctx context.Context, start, end time.Time, userID int, nameFilter string) ([]models.WorkoutRow, error) {
	workouts, err := db.QueryWorkouts(ctx, start, end, userID, nameFilter)
	if err != nil {
		return nil, err
	}

	// Fetch sessions with 2h padding to catch sessions just outside the range.
	alphaSessions, err := db.QuerySetSessions(ctx, start.Add(-2*time.Hour), end.Add(2*time.Hour), userID)
	if err != nil {
		return nil, err
	}
	if len(alphaSessions) == 0 {
		return workouts, nil
	}

	// Match Alpha sessions to workouts by nearest time within ±2h.
	matched := make(map[int]bool)   // index into workouts
	alphaUsed := make(map[int]bool) // index into alphaSessions

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

	// Create synthetic workouts for unmatched sessions.
	for ai, a := range alphaSessions {
		if alphaUsed[ai] {
			continue
		}
		// Skip if outside the requested range.
		if a.SessionDate.Before(start) || !a.SessionDate.Before(end) {
			continue
		}
		// Skip if name filter is set and doesn't match the synthetic base name.
		if nameFilter != "" && nameFilter != syntheticWorkoutName {
			continue
		}
		sessionEnd := syntheticWorkoutEnd(a)
		source := a.Source
		if source == "" {
			source = "Alpha Progression"
		}
		workouts = append(workouts, models.WorkoutRow{
			ID:               syntheticWorkoutID(a),
			UserID:           userID,
			Name:             syntheticWorkoutName,
			Source:           source,
			Sources:          []string{source},
			StartTime:        a.SessionDate,
			EndTime:          sessionEnd,
			DurationSec:      sessionEnd.Sub(a.SessionDate).Seconds(),
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
