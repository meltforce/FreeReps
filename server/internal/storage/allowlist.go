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

// AllowedMetric represents an entry in the metric allowlist with display metadata.
type AllowedMetric struct {
	MetricName        string  `json:"metric_name"`
	Category          string  `json:"category"`
	Enabled           bool    `json:"enabled"`
	DisplayLabel      string  `json:"display_label"`
	DisplayUnit       string  `json:"display_unit"`
	IsCumulative      bool    `json:"is_cumulative"`
	DisplayMultiplier float64 `json:"display_multiplier"`
}

// GetAllowedMetrics returns all metrics in the allowlist.
func (db *DB) GetAllowedMetrics(ctx context.Context) ([]AllowedMetric, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT metric_name, category, enabled, display_label, display_unit, is_cumulative, display_multiplier
		 FROM metric_allowlist ORDER BY category, metric_name`)
	if err != nil {
		return nil, fmt.Errorf("querying allowlist: %w", err)
	}
	defer rows.Close()

	var result []AllowedMetric
	for rows.Next() {
		var m AllowedMetric
		if err := rows.Scan(&m.MetricName, &m.Category, &m.Enabled,
			&m.DisplayLabel, &m.DisplayUnit, &m.IsCumulative, &m.DisplayMultiplier); err != nil {
			return nil, fmt.Errorf("scanning allowlist: %w", err)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// GetAvailableMetrics returns allowlist entries for metrics the user actually has data for.
func (db *DB) GetAvailableMetrics(ctx context.Context, userID int) ([]AllowedMetric, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT a.metric_name, a.category, a.enabled, a.display_label, a.display_unit, a.is_cumulative, a.display_multiplier
		 FROM metric_allowlist a
		 WHERE a.enabled = true
		   AND EXISTS (SELECT 1 FROM health_metrics h WHERE h.metric_name = a.metric_name AND h.user_id = $1)
		 ORDER BY a.category, a.display_label, a.metric_name`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("querying available metrics: %w", err)
	}
	defer rows.Close()

	var result []AllowedMetric
	for rows.Next() {
		var m AllowedMetric
		if err := rows.Scan(&m.MetricName, &m.Category, &m.Enabled,
			&m.DisplayLabel, &m.DisplayUnit, &m.IsCumulative, &m.DisplayMultiplier); err != nil {
			return nil, fmt.Errorf("scanning available metric: %w", err)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// GetAllowlistCategories returns distinct categories from the metric allowlist,
// excluding source-exclusive categories (like "oura") where dedup doesn't apply.
func (db *DB) GetAllowlistCategories(ctx context.Context) ([]string, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT DISTINCT category FROM metric_allowlist WHERE category NOT IN ('oura') ORDER BY category`)
	if err != nil {
		return nil, fmt.Errorf("querying allowlist categories: %w", err)
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, fmt.Errorf("scanning category: %w", err)
		}
		result = append(result, c)
	}
	return result, rows.Err()
}
