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
	Visible           bool    `json:"visible"`
}

// defaultVisibleMetrics is the starter set for users who haven't customized visibility.
var defaultVisibleMetrics = map[string]bool{
	"heart_rate":              true,
	"resting_heart_rate":      true,
	"heart_rate_variability":  true,
	"blood_oxygen_saturation": true,
	"respiratory_rate":        true,
	"vo2_max":                 true,
	"weight_body_mass":        true,
	"body_fat_percentage":     true,
	"active_energy":           true,
	"basal_energy_burned":     true,
	"apple_exercise_time":     true,
	"step_count":              true,
	"flights_climbed":         true,
	"sleep_analysis":          true,
	// Oura (visible by default if user has data)
	"oura_readiness_score": true,
	"oura_sleep_score":     true,
	"oura_activity_score":  true,
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

// GetAvailableMetrics returns allowlist entries for metrics the user actually has data for,
// with per-user visibility resolved (override → default set → false).
func (db *DB) GetAvailableMetrics(ctx context.Context, userID int) ([]AllowedMetric, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT a.metric_name, a.category, a.enabled, a.display_label, a.display_unit, a.is_cumulative, a.display_multiplier,
		        v.visible
		 FROM metric_allowlist a
		 LEFT JOIN user_metric_visibility v ON v.metric_name = a.metric_name AND v.user_id = $1
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
		var visOverride *bool
		if err := rows.Scan(&m.MetricName, &m.Category, &m.Enabled,
			&m.DisplayLabel, &m.DisplayUnit, &m.IsCumulative, &m.DisplayMultiplier,
			&visOverride); err != nil {
			return nil, fmt.Errorf("scanning available metric: %w", err)
		}
		if visOverride != nil {
			m.Visible = *visOverride
		} else {
			m.Visible = defaultVisibleMetrics[m.MetricName]
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// SaveMetricVisibility saves per-user visibility overrides for multiple metrics.
func (db *DB) SaveMetricVisibility(ctx context.Context, userID int, visibility map[string]bool) error {
	for name, visible := range visibility {
		_, err := db.Pool.Exec(ctx,
			`INSERT INTO user_metric_visibility (user_id, metric_name, visible)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (user_id, metric_name) DO UPDATE SET visible = EXCLUDED.visible`,
			userID, name, visible)
		if err != nil {
			return fmt.Errorf("saving metric visibility: %w", err)
		}
	}
	return nil
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
