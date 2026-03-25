package storage

import (
	"context"
	"fmt"
)

// SourcePriorityRule is a per-user, per-category source priority configuration.
// Category "_default" is the global fallback.
type SourcePriorityRule struct {
	UserID   int      `json:"user_id"`
	Category string   `json:"category"`
	Sources  []string `json:"sources"`
}

// GetSourcePriorities returns all source priority rules for a user.
func (db *DB) GetSourcePriorities(ctx context.Context, userID int) ([]SourcePriorityRule, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT user_id, category, sources FROM source_priority WHERE user_id = $1 ORDER BY category`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("querying source priorities: %w", err)
	}
	defer rows.Close()

	var result []SourcePriorityRule
	for rows.Next() {
		var r SourcePriorityRule
		if err := rows.Scan(&r.UserID, &r.Category, &r.Sources); err != nil {
			return nil, fmt.Errorf("scanning source priority: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// UpsertSourcePriority saves a source priority rule for a user and category.
func (db *DB) UpsertSourcePriority(ctx context.Context, userID int, category string, sources []string) error {
	_, err := db.Pool.Exec(ctx,
		`INSERT INTO source_priority (user_id, category, sources)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, category) DO UPDATE SET sources = EXCLUDED.sources`,
		userID, category, sources)
	if err != nil {
		return fmt.Errorf("upserting source priority: %w", err)
	}
	return nil
}

// DeleteSourcePriority removes a category override for a user (falls back to _default).
func (db *DB) DeleteSourcePriority(ctx context.Context, userID int, category string) error {
	_, err := db.Pool.Exec(ctx,
		`DELETE FROM source_priority WHERE user_id = $1 AND category = $2`,
		userID, category)
	if err != nil {
		return fmt.Errorf("deleting source priority: %w", err)
	}
	return nil
}

// GetDistinctSources returns all distinct source values from health_metrics for a user.
func (db *DB) GetDistinctSources(ctx context.Context, userID int) ([]string, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT DISTINCT source FROM health_metrics WHERE user_id = $1 ORDER BY source`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("querying distinct sources: %w", err)
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("scanning source: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// ResolveSourcePriority returns the source priority list for a given user and category.
// Falls back to the user's "_default" rule, then to db.SourcePriority (config global).
func (db *DB) ResolveSourcePriority(ctx context.Context, userID int, category string) []string {
	// Try category-specific rule first.
	var sources []string
	err := db.Pool.QueryRow(ctx,
		`SELECT sources FROM source_priority WHERE user_id = $1 AND category = $2`,
		userID, category).Scan(&sources)
	if err == nil {
		return sources
	}

	// Fall back to _default.
	err = db.Pool.QueryRow(ctx,
		`SELECT sources FROM source_priority WHERE user_id = $1 AND category = '_default'`,
		userID).Scan(&sources)
	if err == nil {
		return sources
	}

	// Fall back to global config.
	return db.SourcePriority
}

// ResolveSourcePriorityForMetric looks up the category for a metric name and
// resolves the source priority for that category.
func (db *DB) ResolveSourcePriorityForMetric(ctx context.Context, userID int, metricName string) []string {
	var category string
	err := db.Pool.QueryRow(ctx,
		`SELECT category FROM metric_allowlist WHERE metric_name = $1`,
		metricName).Scan(&category)
	if err != nil {
		return db.SourcePriority
	}
	return db.ResolveSourcePriority(ctx, userID, category)
}
