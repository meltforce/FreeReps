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

// InsertHealthMetrics batch-inserts health metric rows. Returns the number actually inserted
// (skipped duplicates via ON CONFLICT DO NOTHING).
func (db *DB) InsertHealthMetrics(ctx context.Context, rows []models.HealthMetricRow) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	query := `INSERT INTO health_metrics (time, user_id, metric_name, source, units, qty, min_val, avg_val, max_val, systolic, diastolic)
VALUES `
	args := make([]any, 0, len(rows)*11)
	valueStrings := make([]string, 0, len(rows))

	for i, r := range rows {
		base := i * 11
		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9, base+10, base+11,
		))
		args = append(args, r.Time, r.UserID, r.MetricName, r.Source, r.Units,
			r.Qty, r.MinVal, r.AvgVal, r.MaxVal, r.Systolic, r.Diastolic)
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
		`SELECT time, user_id, metric_name, source, units, qty, min_val, avg_val, max_val, systolic, diastolic
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
		`SELECT DISTINCT ON (metric_name) time, user_id, metric_name, source, units, qty, min_val, avg_val, max_val, systolic, diastolic
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
func (db *DB) GetTimeSeries(ctx context.Context, metricName string, start, end time.Time, bucketSize string, userID int) ([]TimeSeriesPoint, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT time_bucket($1::interval, time) AS bucket,
		        AVG(COALESCE(qty, avg_val)) AS avg_val,
		        MIN(COALESCE(qty, min_val)) AS min_val,
		        MAX(COALESCE(qty, max_val)) AS max_val,
		        COUNT(*) AS count
		 FROM health_metrics
		 WHERE metric_name = $2 AND time >= $3 AND time < $4 AND user_id = $5
		 GROUP BY bucket
		 ORDER BY bucket ASC`,
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
	row := db.Pool.QueryRow(ctx,
		`SELECT AVG(COALESCE(qty, avg_val)),
		        MIN(COALESCE(qty, min_val)),
		        MAX(COALESCE(qty, max_val)),
		        STDDEV_POP(COALESCE(qty, avg_val)),
		        COUNT(*)
		 FROM health_metrics
		 WHERE metric_name = $1 AND time >= $2 AND time < $3 AND user_id = $4`,
		metricName, start, end, userID)

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
func (db *DB) GetCorrelation(ctx context.Context, xMetric, yMetric string, start, end time.Time, bucket string, userID int) (*CorrelationResult, error) {
	rows, err := db.Pool.Query(ctx,
		`WITH x AS (
			SELECT time_bucket($1::interval, time) AS bucket,
			       AVG(COALESCE(qty, avg_val)) AS val
			FROM health_metrics
			WHERE metric_name = $2 AND time >= $4 AND time < $5 AND user_id = $6
			GROUP BY bucket
		), y AS (
			SELECT time_bucket($1::interval, time) AS bucket,
			       AVG(COALESCE(qty, avg_val)) AS val
			FROM health_metrics
			WHERE metric_name = $3 AND time >= $4 AND time < $5 AND user_id = $6
			GROUP BY bucket
		)
		SELECT x.bucket, x.val, y.val
		FROM x JOIN y ON x.bucket = y.bucket
		ORDER BY x.bucket ASC`,
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
			&r.Qty, &r.MinVal, &r.AvgVal, &r.MaxVal, &r.Systolic, &r.Diastolic); err != nil {
			return nil, fmt.Errorf("scanning health metric row: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
