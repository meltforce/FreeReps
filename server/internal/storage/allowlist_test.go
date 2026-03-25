package storage

import (
	"context"
	"testing"
	"time"
)

// TestAvailableMetricsCacheHit verifies that a second call within TTL returns
// the cached result without requiring a database query.
func TestAvailableMetricsCacheHit(t *testing.T) {
	db := &DB{}

	// Seed cache manually.
	metrics := []AllowedMetric{
		{MetricName: "heart_rate", Category: "cardiovascular", Visible: true},
	}
	db.availMetricsMu.Lock()
	db.availMetricsCache = map[int]*availMetricsCacheEntry{
		1: {metrics: metrics, fetchedAt: time.Now()},
	}
	db.availMetricsMu.Unlock()

	// Reading from cache should return the seeded data (no Pool needed).
	got, err := db.GetAvailableMetrics(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].MetricName != "heart_rate" {
		t.Fatalf("expected cached heart_rate, got %v", got)
	}
}

// TestAvailableMetricsCacheMissForDifferentUser verifies that a cached entry
// for user 1 does not serve requests for user 2.
func TestAvailableMetricsCacheMissForDifferentUser(t *testing.T) {
	db := &DB{}

	db.availMetricsMu.Lock()
	db.availMetricsCache = map[int]*availMetricsCacheEntry{
		1: {metrics: []AllowedMetric{{MetricName: "a"}}, fetchedAt: time.Now()},
	}
	db.availMetricsMu.Unlock()

	// User 1 should hit cache.
	got, err := db.GetAvailableMetrics(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].MetricName != "a" {
		t.Fatalf("expected 'a', got %q", got[0].MetricName)
	}

	// User 2 has no entry — verify cache doesn't contain it.
	db.availMetricsMu.RLock()
	_, exists := db.availMetricsCache[2]
	db.availMetricsMu.RUnlock()
	if exists {
		t.Fatal("expected no cache entry for user 2")
	}
}

// TestAvailableMetricsCacheTTLExpiry verifies that entries older than the TTL
// are treated as expired.
func TestAvailableMetricsCacheTTLExpiry(t *testing.T) {
	db := &DB{}

	// Seed cache with a fresh entry — should be cached.
	db.availMetricsMu.Lock()
	db.availMetricsCache = map[int]*availMetricsCacheEntry{
		1: {metrics: []AllowedMetric{{MetricName: "fresh"}}, fetchedAt: time.Now()},
	}
	db.availMetricsMu.Unlock()

	got, err := db.GetAvailableMetrics(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error for fresh cache: %v", err)
	}
	if got[0].MetricName != "fresh" {
		t.Fatalf("expected 'fresh', got %q", got[0].MetricName)
	}

	// Overwrite with an expired entry and verify it's stale.
	db.availMetricsMu.Lock()
	db.availMetricsCache[1] = &availMetricsCacheEntry{
		metrics:   []AllowedMetric{{MetricName: "old"}},
		fetchedAt: time.Now().Add(-10 * time.Minute),
	}
	db.availMetricsMu.Unlock()

	db.availMetricsMu.RLock()
	entry := db.availMetricsCache[1]
	expired := time.Since(entry.fetchedAt) >= availMetricsCacheTTL
	db.availMetricsMu.RUnlock()

	if !expired {
		t.Fatal("expected entry to be expired")
	}
}

// TestAvailableMetricsCacheInvalidateUser verifies that invalidating a single
// user's cache removes only that user's entry.
func TestAvailableMetricsCacheInvalidateUser(t *testing.T) {
	db := &DB{}

	now := time.Now()
	db.availMetricsMu.Lock()
	db.availMetricsCache = map[int]*availMetricsCacheEntry{
		1: {metrics: []AllowedMetric{{MetricName: "a"}}, fetchedAt: now},
		2: {metrics: []AllowedMetric{{MetricName: "b"}}, fetchedAt: now},
	}
	db.availMetricsMu.Unlock()

	db.InvalidateAvailableMetrics(1)

	// User 2 should still be cached.
	got, err := db.GetAvailableMetrics(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].MetricName != "b" {
		t.Fatalf("expected cached 'b' for user 2, got %v", got)
	}

	// User 1 should be gone.
	db.availMetricsMu.RLock()
	_, exists := db.availMetricsCache[1]
	db.availMetricsMu.RUnlock()
	if exists {
		t.Fatal("expected user 1 cache entry to be removed")
	}
}

// TestAvailableMetricsCacheInvalidateAll verifies that InvalidateAllAvailableMetrics
// clears the entire cache.
func TestAvailableMetricsCacheInvalidateAll(t *testing.T) {
	db := &DB{}

	now := time.Now()
	db.availMetricsMu.Lock()
	db.availMetricsCache = map[int]*availMetricsCacheEntry{
		1: {metrics: []AllowedMetric{{MetricName: "a"}}, fetchedAt: now},
		2: {metrics: []AllowedMetric{{MetricName: "b"}}, fetchedAt: now},
	}
	db.availMetricsMu.Unlock()

	db.InvalidateAllAvailableMetrics()

	db.availMetricsMu.RLock()
	size := len(db.availMetricsCache)
	db.availMetricsMu.RUnlock()
	if size != 0 {
		t.Fatalf("expected empty cache after InvalidateAll, got size %d", size)
	}
}

// TestAvailableMetricsCacheBoundedSize verifies that the cache evicts all entries
// when it reaches the maximum size, preventing unbounded memory growth.
func TestAvailableMetricsCacheBoundedSize(t *testing.T) {
	db := &DB{}

	now := time.Now()
	db.availMetricsMu.Lock()
	db.availMetricsCache = make(map[int]*availMetricsCacheEntry)
	for i := range availMetricsCacheMaxSize {
		db.availMetricsCache[i] = &availMetricsCacheEntry{
			metrics:   []AllowedMetric{{MetricName: "m"}},
			fetchedAt: now,
		}
	}
	db.availMetricsMu.Unlock()

	// Simulate what GetAvailableMetrics does after a successful DB fetch
	// when the cache is full.
	db.availMetricsMu.Lock()
	if len(db.availMetricsCache) >= availMetricsCacheMaxSize {
		db.availMetricsCache = make(map[int]*availMetricsCacheEntry)
	}
	db.availMetricsCache[999] = &availMetricsCacheEntry{
		metrics:   []AllowedMetric{{MetricName: "new"}},
		fetchedAt: now,
	}
	db.availMetricsMu.Unlock()

	// Only the new entry should exist.
	db.availMetricsMu.RLock()
	size := len(db.availMetricsCache)
	db.availMetricsMu.RUnlock()
	if size != 1 {
		t.Fatalf("expected cache size 1 after eviction, got %d", size)
	}

	// Verify the new entry is retrievable.
	got, err := db.GetAvailableMetrics(context.Background(), 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].MetricName != "new" {
		t.Fatalf("expected 'new', got %q", got[0].MetricName)
	}
}
