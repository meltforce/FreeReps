package storage

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/claude/freereps/internal/models"
	"github.com/jackc/pgx/v5"
)

// sourcePriorityCaseSQL generates a SQL CASE expression that maps source values
// to priority numbers. Lower numbers = higher priority. Sources not in the list
// get the lowest priority. Named sources use prefix matching (e.g. "Apple Watch"
// matches "Apple Watch Series 9"); empty string uses exact match.
// Returns "1" if no priorities are configured (all sources equal = no-op dedup).
func sourcePriorityCaseSQL(priorities []string) string {
	if len(priorities) == 0 {
		return "1"
	}
	var b strings.Builder
	b.WriteString("CASE ")
	for i, src := range priorities {
		if src == "" {
			fmt.Fprintf(&b, "WHEN source = '' THEN %d ", i+1)
		} else {
			fmt.Fprintf(&b, "WHEN source LIKE '%s%%' THEN %d ", src, i+1)
		}
	}
	fmt.Fprintf(&b, "ELSE %d END", len(priorities)+1)
	return b.String()
}

// dedupCTE returns a WITH clause that deduplicates health_metrics at a fixed
// 5-minute granularity using source priority. The CTE selects all columns plus
// a row number (rn) partitioned by 5-minute time buckets. Callers should filter
// with "WHERE rn = 1" to keep only the highest-priority source per bucket.
func dedupCTE(priorities []string, metricParam, startParam, endParam, userIDParam string) string {
	priorityExpr := sourcePriorityCaseSQL(priorities)
	return fmt.Sprintf(
		`WITH deduped AS (
			SELECT *, ROW_NUMBER() OVER (
				PARTITION BY time_bucket('5 minutes', time)
				ORDER BY %s
			) AS rn
			FROM health_metrics
			WHERE metric_name = %s AND time >= %s AND time < %s AND user_id = %s
		) `, priorityExpr, metricParam, startParam, endParam, userIDParam)
}

// dedupCTEMultiMetric returns a dedup CTE for queries that span multiple metrics
// (e.g. GetDailySums). Partitions by metric_name in addition to time bucket.
func dedupCTEMultiMetric(priorities []string, userIDParam, inClause string) string {
	priorityExpr := sourcePriorityCaseSQL(priorities)
	return fmt.Sprintf(
		`WITH deduped AS (
			SELECT *, ROW_NUMBER() OVER (
				PARTITION BY metric_name, time_bucket('5 minutes', time)
				ORDER BY %s
			) AS rn
			FROM health_metrics
			WHERE user_id = %s AND metric_name IN (%s)
		) `, priorityExpr, userIDParam, inClause)
}

// cumulativeMetrics are metrics that should be summed (not averaged) when aggregating.
var cumulativeMetrics = map[string]bool{
	"active_energy":                true,
	"basal_energy_burned":          true,
	"apple_exercise_time":          true,
	"step_count":                   true,
	"distance_walking_running":     true,
	"distance_cycling":             true,
	"distance_swimming":            true,
	"distance_wheelchair":          true,
	"flights_climbed":              true,
	"apple_move_time":              true,
	"apple_stand_time":             true,
	"push_count":                   true,
	"swimming_stroke_count":        true,
	"distance_downhill_snow_sports": true,
}

// maxParamsPerBatch is the PostgreSQL extended protocol parameter limit (65535)
// divided by 12 parameters per row, with headroom.
const maxRowsPerBatch = 5000

// InsertHealthMetrics batch-inserts health metric rows. Returns the number actually inserted
// (skipped duplicates via ON CONFLICT DO NOTHING).
func (db *DB) InsertHealthMetrics(ctx context.Context, rows []models.HealthMetricRow) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	var totalInserted int64
	for start := 0; start < len(rows); start += maxRowsPerBatch {
		end := start + maxRowsPerBatch
		if end > len(rows) {
			end = len(rows)
		}
		inserted, err := db.insertHealthMetricsBatch(ctx, rows[start:end])
		if err != nil {
			return totalInserted, err
		}
		totalInserted += inserted
	}
	return totalInserted, nil
}

func (db *DB) insertHealthMetricsBatch(ctx context.Context, rows []models.HealthMetricRow) (int64, error) {
	query := `INSERT INTO health_metrics (time, user_id, metric_name, source, units, qty, min_val, avg_val, max_val, systolic, diastolic, source_uuid)
VALUES `
	args := make([]any, 0, len(rows)*12)
	valueStrings := make([]string, 0, len(rows))

	for i, r := range rows {
		base := i * 12
		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9, base+10, base+11, base+12,
		))
		args = append(args, r.Time, r.UserID, r.MetricName, r.Source, r.Units,
			r.Qty, r.MinVal, r.AvgVal, r.MaxVal, r.Systolic, r.Diastolic, r.SourceUUID)
	}

	query += strings.Join(valueStrings, ",") + " ON CONFLICT DO NOTHING"

	tag, err := db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("inserting health metrics: %w", err)
	}
	return tag.RowsAffected(), nil
}

// QueryHealthMetrics retrieves health metrics by name and time range.
func (db *DB) QueryHealthMetrics(ctx context.Context, metricName string, start, end time.Time, userID int) ([]models.HealthMetricRow, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT time, user_id, metric_name, source, units, qty, min_val, avg_val, max_val, systolic, diastolic, source_uuid
		 FROM health_metrics
		 WHERE metric_name = $1 AND time >= $2 AND time < $3 AND user_id = $4
		 ORDER BY time ASC`,
		metricName, start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying health metrics: %w", err)
	}
	defer rows.Close()

	return scanHealthMetricRows(rows)
}

// GetLatestMetrics returns the most recent data point for each metric.
func (db *DB) GetLatestMetrics(ctx context.Context, userID int) ([]models.HealthMetricRow, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT DISTINCT ON (metric_name) time, user_id, metric_name, source, units, qty, min_val, avg_val, max_val, systolic, diastolic, source_uuid
		 FROM health_metrics
		 WHERE user_id = $1
		 ORDER BY metric_name, time DESC`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("querying latest metrics: %w", err)
	}
	defer rows.Close()

	return scanHealthMetricRows(rows)
}

// GetTimeSeries returns aggregated time-series data using time_bucket.
// bucketSize should be a PostgreSQL interval like '1 day', '1 hour'.
// Cumulative metrics (active_energy, basal_energy_burned, apple_exercise_time)
// use SUM; all others use AVG.
func (db *DB) GetTimeSeries(ctx context.Context, metricName string, start, end time.Time, bucketSize string, userID int) ([]TimeSeriesPoint, error) {
	aggFunc := "AVG"
	if cumulativeMetrics[metricName] {
		aggFunc = "SUM"
	}
	priorities := db.ResolveSourcePriorityForMetric(ctx, userID, metricName)
	cte := dedupCTE(priorities, "$2", "$3", "$4", "$5")
	query := fmt.Sprintf(
		`%sSELECT time_bucket($1::interval, time) AS bucket,
		        %s(COALESCE(qty, avg_val)) AS avg_val,
		        MIN(COALESCE(qty, min_val)) AS min_val,
		        MAX(COALESCE(qty, max_val)) AS max_val,
		        COUNT(*) AS count
		 FROM deduped WHERE rn = 1
		 GROUP BY bucket
		 ORDER BY bucket ASC`, cte, aggFunc)
	rows, err := db.Pool.Query(ctx, query,
		bucketSize, metricName, start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying time series: %w", err)
	}
	defer rows.Close()

	var result []TimeSeriesPoint
	for rows.Next() {
		var p TimeSeriesPoint
		if err := rows.Scan(&p.Time, &p.Avg, &p.Min, &p.Max, &p.Count); err != nil {
			return nil, fmt.Errorf("scanning time series: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// TimeSeriesPoint is an aggregated data point.
type TimeSeriesPoint struct {
	Time  time.Time `json:"time"`
	Avg   *float64  `json:"avg"`
	Min   *float64  `json:"min"`
	Max   *float64  `json:"max"`
	Count int64     `json:"count"`
}

// DailySum represents the sum of a cumulative metric for the current day.
type DailySum struct {
	MetricName string  `json:"MetricName"`
	Units      string  `json:"Units"`
	Total      float64 `json:"Total"`
}

// GetDailySums returns summed values for the most recent day with data for cumulative metrics.
// Uses the latest available data day rather than today, so historical data still shows values.
func (db *DB) GetDailySums(ctx context.Context, userID int, metricNames []string) ([]DailySum, error) {
	if len(metricNames) == 0 {
		return nil, nil
	}

	// Build IN clause
	params := make([]string, len(metricNames))
	args := make([]any, 0, len(metricNames)+1)
	args = append(args, userID)
	for i, name := range metricNames {
		params[i] = fmt.Sprintf("$%d", i+2)
		args = append(args, name)
	}

	inClause := strings.Join(params, ",")
	// DailySums spans multiple metrics (potentially different categories).
	// Use the user's _default priority.
	priorities := db.ResolveSourcePriority(ctx, userID, "_default")
	cte := dedupCTEMultiMetric(priorities, "$1", inClause)

	query := fmt.Sprintf(
		`%sSELECT metric_name,
		        COALESCE(MAX(units), '') as units,
		        COALESCE(SUM(COALESCE(qty, avg_val, 0)), 0) as total
		 FROM deduped
		 WHERE rn = 1
		   AND time >= (SELECT date_trunc('day', MAX(time)) FROM deduped WHERE rn = 1)
		 GROUP BY metric_name`,
		cte)

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying daily sums: %w", err)
	}
	defer rows.Close()

	var result []DailySum
	for rows.Next() {
		var s DailySum
		if err := rows.Scan(&s.MetricName, &s.Units, &s.Total); err != nil {
			return nil, fmt.Errorf("scanning daily sum: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// MetricStats holds aggregate statistics for a single metric over a time range.
type MetricStats struct {
	Metric string   `json:"metric"`
	Avg    *float64 `json:"avg"`
	Min    *float64 `json:"min"`
	Max    *float64 `json:"max"`
	StdDev *float64 `json:"stddev"`
	Count  int64    `json:"count"`
}

// GetMetricStats returns aggregate statistics for a metric over a time range.
func (db *DB) GetMetricStats(ctx context.Context, metricName string, start, end time.Time, userID int) (*MetricStats, error) {
	priorities := db.ResolveSourcePriorityForMetric(ctx, userID, metricName)
	cte := dedupCTE(priorities, "$1", "$2", "$3", "$4")
	query := fmt.Sprintf(
		`%sSELECT AVG(COALESCE(qty, avg_val)),
		        MIN(COALESCE(qty, min_val)),
		        MAX(COALESCE(qty, max_val)),
		        STDDEV_POP(COALESCE(qty, avg_val)),
		        COUNT(*)
		 FROM deduped WHERE rn = 1`, cte)
	row := db.Pool.QueryRow(ctx, query, metricName, start, end, userID)

	stats := &MetricStats{Metric: metricName}
	if err := row.Scan(&stats.Avg, &stats.Min, &stats.Max, &stats.StdDev, &stats.Count); err != nil {
		return nil, fmt.Errorf("querying metric stats: %w", err)
	}
	return stats, nil
}

// CorrelationPoint is a time-aligned pair of metric values.
type CorrelationPoint struct {
	Time time.Time `json:"time"`
	X    *float64  `json:"x"`
	Y    *float64  `json:"y"`
}

// CorrelationResult holds paired data and a Pearson correlation coefficient.
type CorrelationResult struct {
	Points   []CorrelationPoint `json:"points"`
	PearsonR *float64           `json:"pearson_r"`
	Count    int64              `json:"count"`
}

// GetCorrelation joins two metrics on time buckets and computes their Pearson correlation.
// Uses SUM for cumulative metrics, AVG for all others.
func (db *DB) GetCorrelation(ctx context.Context, xMetric, yMetric string, start, end time.Time, bucket string, userID int) (*CorrelationResult, error) {
	xAgg := "AVG"
	if cumulativeMetrics[xMetric] {
		xAgg = "SUM"
	}
	yAgg := "AVG"
	if cumulativeMetrics[yMetric] {
		yAgg = "SUM"
	}
	// For correlation, use the priority for the X metric's category.
	priorities := db.ResolveSourcePriorityForMetric(ctx, userID, xMetric)
	priorityExpr := sourcePriorityCaseSQL(priorities)
	query := fmt.Sprintf(
		`WITH x_deduped AS (
			SELECT *, ROW_NUMBER() OVER (
				PARTITION BY time_bucket('5 minutes', time)
				ORDER BY %s
			) AS rn
			FROM health_metrics
			WHERE metric_name = $2 AND time >= $4 AND time < $5 AND user_id = $6
		), y_deduped AS (
			SELECT *, ROW_NUMBER() OVER (
				PARTITION BY time_bucket('5 minutes', time)
				ORDER BY %s
			) AS rn
			FROM health_metrics
			WHERE metric_name = $3 AND time >= $4 AND time < $5 AND user_id = $6
		), x AS (
			SELECT time_bucket($1::interval, time) AS bucket,
			       %s(COALESCE(qty, avg_val)) AS val
			FROM x_deduped WHERE rn = 1
			GROUP BY bucket
		), y AS (
			SELECT time_bucket($1::interval, time) AS bucket,
			       %s(COALESCE(qty, avg_val)) AS val
			FROM y_deduped WHERE rn = 1
			GROUP BY bucket
		)
		SELECT x.bucket, x.val, y.val
		FROM x JOIN y ON x.bucket = y.bucket
		ORDER BY x.bucket ASC`, priorityExpr, priorityExpr, xAgg, yAgg)
	rows, err := db.Pool.Query(ctx, query,
		bucket, xMetric, yMetric, start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying correlation: %w", err)
	}
	defer rows.Close()

	var points []CorrelationPoint
	for rows.Next() {
		var p CorrelationPoint
		if err := rows.Scan(&p.Time, &p.X, &p.Y); err != nil {
			return nil, fmt.Errorf("scanning correlation point: %w", err)
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := &CorrelationResult{
		Points: points,
		Count:  int64(len(points)),
	}

	// Compute Pearson R
	if len(points) >= 3 {
		var sumX, sumY, sumXY, sumX2, sumY2 float64
		var n float64
		for _, p := range points {
			if p.X != nil && p.Y != nil {
				x, y := *p.X, *p.Y
				sumX += x
				sumY += y
				sumXY += x * y
				sumX2 += x * x
				sumY2 += y * y
				n++
			}
		}
		if n >= 3 {
			denom := (n*sumX2 - sumX*sumX) * (n*sumY2 - sumY*sumY)
			if denom > 0 {
				r := (n*sumXY - sumX*sumY) / math.Sqrt(denom)
				result.PearsonR = &r
			}
		}
	}

	return result, nil
}

func scanHealthMetricRows(rows pgx.Rows) ([]models.HealthMetricRow, error) {
	var result []models.HealthMetricRow
	for rows.Next() {
		var r models.HealthMetricRow
		if err := rows.Scan(&r.Time, &r.UserID, &r.MetricName, &r.Source, &r.Units,
			&r.Qty, &r.MinVal, &r.AvgVal, &r.MaxVal, &r.Systolic, &r.Diastolic, &r.SourceUUID); err != nil {
			return nil, fmt.Errorf("scanning health metric row: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
