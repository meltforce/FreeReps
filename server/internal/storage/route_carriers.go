package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// A route carrier is the workout the iOS app sends in its route pass: id, name,
// start, end and the route, without source and without measurements. For an
// Apple workout it meets the row of the same id. A workout a provider wrote to
// Apple Health and FreeReps also syncs directly (Oura) is stored under the
// provider's id, so the carrier would become a second, empty workout holding
// the route. Its twin is that directly synced row: a source, no source bundle,
// and the same start and end.

// twinTolerance covers the sub-second part HealthKit drops from the times a
// provider reports.
const twinTolerance = "2 seconds"

const twinMatch = `t.user_id = c.user_id AND t.id <> c.id
	AND t.source <> '' AND t.source_bundle = ''
	AND t.start_time BETWEEN c.start_time - interval '` + twinTolerance + `' AND c.start_time + interval '` + twinTolerance + `'
	AND t.end_time   BETWEEN c.end_time   - interval '` + twinTolerance + `' AND c.end_time   + interval '` + twinTolerance + `'`

// RouteCarrierTwin returns the directly synced workout a route carrier with
// this id and these times belongs to. It reports none when a workout of the
// carrier's id exists: that row takes the route.
func (db *DB) RouteCarrierTwin(ctx context.Context, userID int, id uuid.UUID, start, end time.Time) (uuid.UUID, bool, error) {
	var twin uuid.UUID
	err := db.Pool.QueryRow(ctx,
		`SELECT t.id
		   FROM (SELECT $1::int AS user_id, $2::uuid AS id, $3::timestamptz AS start_time, $4::timestamptz AS end_time) c
		   JOIN workouts t ON `+twinMatch+`
		  WHERE NOT EXISTS (SELECT 1 FROM workouts w WHERE w.user_id = c.user_id AND w.id = c.id)
		  ORDER BY t.id LIMIT 1`,
		userID, id, start, end).Scan(&twin)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("finding the twin of a route carrier: %w", err)
	}
	return twin, true, nil
}

// MergeRouteCarriers moves the routes of the user's stored route carriers to
// their twins and deletes the carriers; it returns how many it merged. A
// carrier is stored when its route arrives before the provider's sync has
// delivered the twin, so the provider's sync calls this after it inserts
// workouts.
func (db *DB) MergeRouteCarriers(ctx context.Context, userID int) (int, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT c.id, t.id, c.start_time, c.end_time
		   FROM workouts c
		   JOIN LATERAL (SELECT t.id FROM workouts t WHERE `+twinMatch+` ORDER BY t.id LIMIT 1) t ON true
		  WHERE c.user_id = $1 AND c.source = '' AND c.source_bundle = ''
		    AND c.active_energy_burned IS NULL AND c.total_energy IS NULL
		    AND c.distance IS NULL AND c.avg_heart_rate IS NULL`,
		userID)
	if err != nil {
		return 0, fmt.Errorf("listing route carriers: %w", err)
	}
	type pair struct {
		carrier, twin uuid.UUID
		start, end    time.Time
	}
	var pairs []pair
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.carrier, &p.twin, &p.start, &p.end); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scanning route carrier: %w", err)
		}
		pairs = append(pairs, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("listing route carriers: %w", err)
	}

	merged := 0
	for _, p := range pairs {
		// The time bounds keep each statement to the chunks of that workout.
		from, to := p.start.Add(-time.Hour), p.end.Add(time.Hour)
		err := pgx.BeginFunc(ctx, db.Pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx,
				`INSERT INTO workout_routes (time, workout_id, user_id, latitude, longitude, altitude, speed, course, horizontal_accuracy, vertical_accuracy)
				 SELECT time, $2, user_id, latitude, longitude, altitude, speed, course, horizontal_accuracy, vertical_accuracy
				   FROM workout_routes
				  WHERE workout_id = $1 AND user_id = $3 AND time BETWEEN $4 AND $5
				 ON CONFLICT DO NOTHING`,
				p.carrier, p.twin, userID, from, to); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`DELETE FROM workout_routes WHERE workout_id = $1 AND user_id = $2 AND time BETWEEN $3 AND $4`,
				p.carrier, userID, from, to); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `DELETE FROM workouts WHERE id = $1 AND user_id = $2`, p.carrier, userID)
			return err
		})
		if err != nil {
			return merged, fmt.Errorf("merging route carrier %s: %w", p.carrier, err)
		}
		merged++
	}
	return merged, nil
}
