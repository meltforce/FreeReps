package storage

import (
	"context"
	"fmt"
	"time"
)

// ZoneCount is the number of heart rate zones. Zone 5 is the top one and the
// only one drawn in the accent.
const ZoneCount = 5

// zoneBounds are the zone edges as fractions of the heart rate reserve, the
// span between resting and maximum heart rate (Karvonen). Zone 1 holds
// everything below the first edge.
//
// The reserve replaced fractions of the maximum on 2026-09-27: an easy run the
// Apple Watch placed in zones 1 to 3 landed in zones 3 and 4 here, because a
// fraction of the maximum puts the lower edges close to a trained resting
// rate. The watch computes its zones from the reserve as well.
var zoneBounds = []float64{0.6, 0.7, 0.8, 0.9}

// restingWindow is the span the resting heart rate is taken from. A month
// smooths out a night of illness or alcohol without lagging a training block.
const restingWindow = 30 * 24 * time.Hour

// ZoneEdges returns the four bpm edges between the five zones. A resting rate
// of 0 means none is known; the edges then reduce to fractions of the maximum,
// which is the reserve formula with a resting rate of zero.
func ZoneEdges(maxHR, restingHR float64) []float64 {
	if maxHR <= 0 {
		return nil
	}
	if restingHR < 0 || restingHR >= maxHR {
		restingHR = 0
	}
	edges := make([]float64, len(zoneBounds))
	for i, f := range zoneBounds {
		edges[i] = restingHR + f*(maxHR-restingHR)
	}
	return edges
}

// WorkoutZones holds one session's share of time per zone, as fractions summing
// to 1. Empty when the session has no heart rate samples.
type WorkoutZones struct {
	WorkoutID string    `json:"workout_id"`
	Shares    []float64 `json:"shares"`
}

// MaxHeartRate carries the figures the zone bands derive from, and where the
// maximum came from — the screen states the source, because the three differ
// in kind.
type MaxHeartRate struct {
	BPM float64 `json:"bpm"`
	// "configured" when the user set it, "estimated" when derived from age,
	// "observed" when taken from the workout history.
	Origin string `json:"origin"`
	// The measured figure, kept whichever origin wins, so the settings screen
	// can offer it as a starting point.
	Observed float64 `json:"observed"`
	// The age-based estimate, 0 when no birth date is stored.
	Estimated float64 `json:"estimated"`
	// Completed years, 0 when no birth date is stored.
	Age int `json:"age"`
	// Median resting heart rate of the last 30 days, 0 when none is stored.
	Resting float64 `json:"resting"`
	// The four bpm edges between the five zones, from BPM and Resting. Empty
	// when BPM is 0.
	ZoneEdges []float64 `json:"zone_edges"`
}

// GetMaxHeartRate returns the heart rate the zone bands are computed from.
// BPM is 0 when none of the three sources yields a figure.
//
// The order is configured, then age-based estimate, then observed:
//
//   - A configured maximum wins because the user knows it, typically from a
//     test the app never saw.
//   - The estimate comes next because it approximates a physiological constant,
//     while the observed figure only describes how hard this person has trained
//     so far — it shifts every band retroactively after one hard session.
//   - An observed rate above the estimate overrides it anyway: the formula
//     carries a 10 to 12 bpm standard deviation, and a rate actually recorded
//     is evidence against an estimate that sits below it.
func (db *DB) GetMaxHeartRate(ctx context.Context, userID int) (MaxHeartRate, error) {
	observed, err := db.observedMaxHeartRate(ctx, userID)
	if err != nil {
		return MaxHeartRate{}, err
	}

	result := MaxHeartRate{Observed: observed}

	birth, hasBirth, err := db.GetBirthDate(ctx, userID)
	if err != nil {
		return MaxHeartRate{}, err
	}
	if hasBirth {
		result.Age = AgeYears(birth, time.Now())
		result.Estimated = EstimatedMaxHeartRate(result.Age)
	}

	var configured float64
	found, err := db.GetPreference(ctx, userID, PrefMaxHeartRate, &configured)
	if err != nil {
		return MaxHeartRate{}, err
	}
	switch {
	case found && configured > 0:
		result.BPM, result.Origin = configured, "configured"
	case result.Estimated > 0 && result.Estimated >= observed:
		result.BPM, result.Origin = result.Estimated, "estimated"
	default:
		result.BPM, result.Origin = observed, "observed"
	}

	result.Resting, err = db.restingHeartRate(ctx, userID, time.Now())
	if err != nil {
		return MaxHeartRate{}, err
	}
	result.ZoneEdges = ZoneEdges(result.BPM, result.Resting)
	return result, nil
}

// restingHeartRate is the median of the daily resting heart rate over the
// restingWindow before now, 0 when none is stored.
//
// Apple Health and Oura both report the metric; from 2026-09-14 to 2026-09-27
// the two differed by a median of 8 bpm per day.
// Apple Health estimates a daytime resting
// rate, Oura reports the lowest rate of the night. One source wins per day by the user's priority for the metric,
// so a day never contributes both figures. The median keeps one feverish day
// from moving every zone edge.
func (db *DB) restingHeartRate(ctx context.Context, userID int, now time.Time) (float64, error) {
	const metric = "resting_heart_rate"
	priorities := db.ResolveSourcePriorityForMetric(ctx, userID, metric)
	rn := winningSourceRN(sourcePriorityCaseSQL(priorities), "time_bucket('1 day', time)")

	var median *float64
	err := db.Pool.QueryRow(ctx, fmt.Sprintf(
		`WITH deduped AS (
			SELECT qty, %s
			FROM health_metrics
			WHERE user_id = $1 AND metric_name = $2
			  AND time >= $3 AND time < $4 AND qty > 0
		)
		SELECT PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY qty)
		FROM deduped WHERE rn = 1`, rn),
		userID, metric, now.Add(-restingWindow), now).Scan(&median)
	if err != nil {
		return 0, fmt.Errorf("querying resting heart rate: %w", err)
	}
	if median == nil {
		return 0, nil
	}
	return *median, nil
}

// observedMaxHeartRate is the 99.9th percentile, not the maximum. A single
// spurious sample — a chest strap dropout reads as 210 bpm — would otherwise
// set every band: at 210 the second zone starts at 126, which puts whole
// strength sessions in zone 1 and makes the bars say nothing. A real maximum
// effort contributes many samples near the top, an artefact contributes one.
func (db *DB) observedMaxHeartRate(ctx context.Context, userID int) (float64, error) {
	var peak *float64
	err := db.Pool.QueryRow(ctx,
		`SELECT PERCENTILE_CONT(0.999) WITHIN GROUP (ORDER BY COALESCE(max_bpm, avg_bpm))
		 FROM workout_heart_rate
		 WHERE user_id = $1 AND COALESCE(max_bpm, avg_bpm) IS NOT NULL`,
		userID).Scan(&peak)
	if err != nil {
		return 0, fmt.Errorf("querying peak heart rate: %w", err)
	}
	if peak == nil {
		return 0, nil
	}
	return *peak, nil
}

// GetWorkoutZones returns the per-zone share of each workout in the range.
//
// Shares are counted by sample rather than by elapsed time. Samples arrive at a
// near-constant cadence within a session, so the two agree closely, and
// counting avoids a window function over every heart rate row in the range —
// this runs for a whole workout list, not one session.
//
// edges are the bpm boundaries from ZoneEdges; without them there are no zones.
func (db *DB) GetWorkoutZones(ctx context.Context, userID int, start, end time.Time, edges []float64) ([]WorkoutZones, error) {
	if len(edges) != ZoneCount-1 {
		return nil, nil
	}

	rows, err := db.Pool.Query(ctx,
		`SELECT hr.workout_id,
		        WIDTH_BUCKET(COALESCE(hr.avg_bpm, hr.max_bpm), $4::float8[]) AS zone,
		        COUNT(*)
		 FROM workout_heart_rate hr
		 JOIN workouts w ON w.id = hr.workout_id
		 WHERE hr.user_id = $1 AND w.start_time >= $2 AND w.start_time < $3
		   AND COALESCE(hr.avg_bpm, hr.max_bpm) IS NOT NULL
		 GROUP BY hr.workout_id, zone`,
		userID, start, end, edges)
	if err != nil {
		return nil, fmt.Errorf("querying workout zones: %w", err)
	}
	defer rows.Close()

	counts := make(map[string][]float64)
	for rows.Next() {
		var id string
		var zone int
		var n int64
		if err := rows.Scan(&id, &zone, &n); err != nil {
			return nil, fmt.Errorf("scanning workout zone: %w", err)
		}
		if _, ok := counts[id]; !ok {
			counts[id] = make([]float64, ZoneCount)
		}
		if zone >= 0 && zone < ZoneCount {
			counts[id][zone] += float64(n)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]WorkoutZones, 0, len(counts))
	for id, c := range counts {
		var total float64
		for _, v := range c {
			total += v
		}
		if total == 0 {
			continue
		}
		shares := make([]float64, ZoneCount)
		for i, v := range c {
			shares[i] = v / total
		}
		result = append(result, WorkoutZones{WorkoutID: id, Shares: shares})
	}
	return result, nil
}
