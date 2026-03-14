package storage

import (
	"context"
	"fmt"
)

// IsMetricAllowed checks if a metric name is in the allowlist and enabled.
func (db *DB) IsMetricAllowed(ctx context.Context, metricName string) (bool, error) {
	var enabled bool
	err := db.Pool.QueryRow(ctx,
		`SELECT enabled FROM metric_allowlist WHERE metric_name = $1`,
		metricName).Scan(&enabled)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return false, nil
		}
		return false, fmt.Errorf("checking metric allowlist: %w", err)
	}
	return enabled, nil
}

// AllowedMetric represents an entry in the metric allowlist.
type AllowedMetric struct {
	MetricName string `json:"metric_name"`
	Category   string `json:"category"`
	Enabled    bool   `json:"enabled"`
}

// GetAllowedMetrics returns all metrics in the allowlist.
func (db *DB) GetAllowedMetrics(ctx context.Context) ([]AllowedMetric, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT metric_name, category, enabled FROM metric_allowlist ORDER BY category, metric_name`)
	if err != nil {
		return nil, fmt.Errorf("querying allowlist: %w", err)
	}
	defer rows.Close()

	var result []AllowedMetric
	for rows.Next() {
		var m AllowedMetric
		if err := rows.Scan(&m.MetricName, &m.Category, &m.Enabled); err != nil {
			return nil, fmt.Errorf("scanning allowlist: %w", err)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
