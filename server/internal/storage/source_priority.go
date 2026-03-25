package storage

import (
	"context"
	"fmt"
	"sync"
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

// --- Priority resolver with caching ---

// metricCategoryCache caches the metric_name → category mapping from the allowlist.
// This is global (same for all users) and rarely changes.
var (
	metricCategoryMap  map[string]string
	metricCategoryOnce sync.Once
)

// loadMetricCategories populates the global metric→category cache.
func (db *DB) loadMetricCategories(ctx context.Context) {
	metricCategoryOnce.Do(func() {
		m := make(map[string]string)
		rows, err := db.Pool.Query(ctx,
			`SELECT metric_name, category FROM metric_allowlist`)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var name, cat string
			if err := rows.Scan(&name, &cat); err == nil {
				m[name] = cat
			}
		}
		metricCategoryMap = m
	})
}

// PriorityResolver resolves source priorities from an in-memory cache.
// Create one per request to avoid repeated DB queries.
type PriorityResolver struct {
	rules    map[string][]string // category → sources
	fallback []string            // config global
}

// NewPriorityResolver loads all source priority rules for a user into memory.
// One DB query, then all resolutions are in-memory.
func (db *DB) NewPriorityResolver(ctx context.Context, userID int) *PriorityResolver {
	db.loadMetricCategories(ctx)

	pr := &PriorityResolver{
		rules:    make(map[string][]string),
		fallback: db.SourcePriority,
	}

	rows, err := db.Pool.Query(ctx,
		`SELECT category, sources FROM source_priority WHERE user_id = $1`, userID)
	if err != nil {
		return pr
	}
	defer rows.Close()

	for rows.Next() {
		var cat string
		var sources []string
		if err := rows.Scan(&cat, &sources); err == nil {
			pr.rules[cat] = sources
		}
	}
	return pr
}

// ForCategory returns the source priority for a category.
func (pr *PriorityResolver) ForCategory(category string) []string {
	if sources, ok := pr.rules[category]; ok {
		return sources
	}
	if sources, ok := pr.rules["_default"]; ok {
		return sources
	}
	return pr.fallback
}

// ForMetric returns the source priority for a metric name by looking up its category.
func (pr *PriorityResolver) ForMetric(metricName string) []string {
	if cat, ok := metricCategoryMap[metricName]; ok {
		return pr.ForCategory(cat)
	}
	return pr.ForCategory("_default")
}

// ResolveSourcePriority returns the source priority list for a given user and category.
// Falls back to the user's "_default" rule, then to db.SourcePriority (config global).
// NOTE: For repeated lookups, prefer NewPriorityResolver to avoid per-call DB queries.
func (db *DB) ResolveSourcePriority(ctx context.Context, userID int, category string) []string {
	var sources []string
	err := db.Pool.QueryRow(ctx,
		`SELECT sources FROM source_priority WHERE user_id = $1 AND category = $2`,
		userID, category).Scan(&sources)
	if err == nil {
		return sources
	}

	err = db.Pool.QueryRow(ctx,
		`SELECT sources FROM source_priority WHERE user_id = $1 AND category = '_default'`,
		userID).Scan(&sources)
	if err == nil {
		return sources
	}

	return db.SourcePriority
}

// ResolveSourcePriorityForMetric looks up the category for a metric name and
// resolves the source priority for that category.
// NOTE: For repeated lookups, prefer NewPriorityResolver to avoid per-call DB queries.
func (db *DB) ResolveSourcePriorityForMetric(ctx context.Context, userID int, metricName string) []string {
	db.loadMetricCategories(ctx)
	if cat, ok := metricCategoryMap[metricName]; ok {
		return db.ResolveSourcePriority(ctx, userID, cat)
	}
	return db.SourcePriority
}
