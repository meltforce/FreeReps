package storage

import (
	"strings"
	"testing"
	"time"
)

// TestSourcePriorityCaseSQL verifies that the SQL CASE expression correctly
// maps source names to priority numbers, ensuring higher-priority sources
// win during deduplication.
func TestSourcePriorityCaseSQL(t *testing.T) {
	tests := []struct {
		name       string
		priorities []string
		wantSQL    string
	}{
		{
			name:       "empty priorities returns constant 1 (no-op dedup)",
			priorities: nil,
			wantSQL:    "1",
		},
		{
			name:       "single named source",
			priorities: []string{"Oura"},
			wantSQL:    "CASE WHEN source LIKE 'Oura%' THEN 1 ELSE 2 END",
		},
		{
			name:       "oura then empty string",
			priorities: []string{"Oura", ""},
			wantSQL:    "CASE WHEN source LIKE 'Oura%' THEN 1 WHEN source = '' THEN 2 ELSE 3 END",
		},
		{
			name:       "three sources with prefix matching",
			priorities: []string{"Oura", "Apple Watch", ""},
			wantSQL:    "CASE WHEN source LIKE 'Oura%' THEN 1 WHEN source LIKE 'Apple Watch%' THEN 2 WHEN source = '' THEN 3 ELSE 4 END",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sourcePriorityCaseSQL(tt.priorities)
			if got != tt.wantSQL {
				t.Errorf("sourcePriorityCaseSQL() =\n  %q\nwant:\n  %q", got, tt.wantSQL)
			}
		})
	}
}

// TestDedupCTE verifies that the generated CTE has the correct structure:
// a WITH clause using time_bucket, ROW_NUMBER, and the right parameter placeholders.
func TestDedupCTE(t *testing.T) {
	cte := dedupCTE([]string{"Oura", ""}, "heart_rate", "$2", "$3", "$4", "$5")

	checks := []string{
		"WITH deduped AS",
		"time_bucket('5 minutes', time)",
		"FIRST_VALUE(source)",
		"LIKE 'Oura%' THEN 1",
		"source = '' THEN 2",
		"metric_name = $2",
		"time >= $3",
		"time < $4",
		"user_id = $5",
	}

	for _, check := range checks {
		if !strings.Contains(cte, check) {
			t.Errorf("dedupCTE missing %q in:\n%s", check, cte)
		}
	}
}

// TestDedupCTEKeepsEverySampleOfTheWinningSource pins the change that made
// cumulative metrics addable: the CTE used to number rows within a window and
// callers kept rn = 1, so a sum over per-second samples became a sum over one
// of them. The predicate now separates sources, not samples.
func TestDedupCTEKeepsEverySampleOfTheWinningSource(t *testing.T) {
	for _, metric := range []string{"step_count", "heart_rate"} {
		cte := dedupCTE([]string{"Oura", ""}, metric, "$2", "$3", "$4", "$5")
		if strings.Contains(cte, "ROW_NUMBER()") {
			t.Errorf("%s: ROW_NUMBER() drops samples of the winning source:\n%s", metric, cte)
		}
		if !strings.Contains(cte, "FIRST_VALUE(source)") {
			t.Errorf("%s: expected the winning source to be selected by FIRST_VALUE:\n%s", metric, cte)
		}
	}
}

// TestDedupCTECumulativeResolvesSourcePerDay covers the front page reporting
// 32361 steps for 2026-09-18 where Apple Health had recorded 16652: Oura writes
// the day's total as a single row, so choosing a source per 5-minute window
// kept that total in its own window and added the hourly Apple blocks around
// it.
func TestDedupCTECumulativeResolvesSourcePerDay(t *testing.T) {
	for _, metric := range []string{"step_count", "active_energy", "flights_climbed"} {
		cte := dedupCTE([]string{"", "Oura"}, metric, "$2", "$3", "$4", "$5")
		if !strings.Contains(cte, "PARTITION BY time_bucket('1 day', time)") {
			t.Errorf("%s: expected a daily partition, got:\n%s", metric, cte)
		}
	}
}

// TestDedupCTENonCumulativeResolvesSourcePerWindow is the other half: a daily
// partition would hand the whole day to Oura whenever Oura reported at all. On
// 2026-09-18 Apple Health held 54 windows in which Oura had no row, covering
// the morning strength session.
func TestDedupCTENonCumulativeResolvesSourcePerWindow(t *testing.T) {
	for _, metric := range []string{"heart_rate", "body_mass", "respiratory_rate"} {
		cte := dedupCTE([]string{"Oura", ""}, metric, "$2", "$3", "$4", "$5")
		if !strings.Contains(cte, "PARTITION BY time_bucket('5 minutes', time)") {
			t.Errorf("%s: expected a 5-minute partition, got:\n%s", metric, cte)
		}
	}
}

// TestMetricStatsQueryCumulativeSums covers the metric page reporting an
// average of 925 steps for a day of 16652: AVG over per-block samples answers a
// question nobody asks of a counter.
func TestMetricStatsQueryCumulativeSums(t *testing.T) {
	for metric := range cumulativeMetrics {
		q := buildMetricStatsQuery([]string{"", "Oura"}, metric, "$1", "$2", "$3", "$4")
		if !strings.Contains(q, "SELECT SUM(COALESCE(qty, avg_val))") {
			t.Errorf("%s: expected the headline aggregate to be SUM, got:\n%s", metric, q)
		}
	}
}

// TestMetricStatsQueryNonCumulativeAveragesWindowsFirst pins the two-stage
// average. On 2026-09-18 six workouts covered 16% of the day and held 77% of
// the heart rate rows, so averaging rows directly would let them carry the
// daily figure.
func TestMetricStatsQueryNonCumulativeAveragesWindowsFirst(t *testing.T) {
	q := buildMetricStatsQuery([]string{"Oura", ""}, "heart_rate", "$1", "$2", "$3", "$4")

	for _, check := range []string{
		"windowed AS",
		"AVG(COALESCE(qty, avg_val)) AS avg_val",
		"GROUP BY sub",
		"SELECT AVG(avg_val)",
	} {
		if !strings.Contains(q, check) {
			t.Errorf("missing %q in:\n%s", check, q)
		}
	}
	// The extremes stay over every row rather than over the window averages.
	if !strings.Contains(q, "MIN(COALESCE(qty, min_val)) AS min_val") {
		t.Errorf("expected MIN over all rows in:\n%s", q)
	}
}

// TestTimeSeriesSelectMatchesStats keeps the series and the statistics beside it
// on the same rule; they were computed differently before.
func TestTimeSeriesSelectMatchesStats(t *testing.T) {
	cumulative := timeSeriesSelectSQL("step_count")
	if !strings.Contains(cumulative, "SUM(COALESCE(qty, avg_val)) AS avg_val") {
		t.Errorf("expected a sum for step_count, got:\n%s", cumulative)
	}
	if strings.Contains(cumulative, "windowed AS") {
		t.Errorf("a cumulative metric needs no window stage:\n%s", cumulative)
	}

	plain := timeSeriesSelectSQL("heart_rate")
	if !strings.Contains(plain, "windowed AS") || !strings.Contains(plain, "AVG(avg_val)") {
		t.Errorf("expected the two-stage average for heart_rate, got:\n%s", plain)
	}
}

// TestPerMetricPriorityExprGroupsSharedLists covers the front page and the
// metric page disagreeing about the same day: a multi-metric query resolved
// every metric with the user's _default priority instead of the priority its
// category carries.
func TestPerMetricPriorityExprGroupsSharedLists(t *testing.T) {
	expr := perMetricPriorityExpr(map[string][]string{
		"step_count":    {"", "Oura"},
		"active_energy": {"", "Oura"},
		"heart_rate":    {"Oura", ""},
		"body_mass":     {"Withings", "Oura"},
	})

	if !strings.Contains(expr, "WHEN metric_name IN ('active_energy','step_count')") {
		t.Errorf("expected metrics with one priority list to share a branch, got:\n%s", expr)
	}
	if strings.Count(expr, "WHEN metric_name IN") != 2 {
		t.Errorf("expected three lists to collapse into two branches and an ELSE, got:\n%s", expr)
	}

	// A single list needs no CASE over metric_name at all.
	single := perMetricPriorityExpr(map[string][]string{"a": {"Oura"}, "b": {"Oura"}})
	if strings.Contains(single, "metric_name") {
		t.Errorf("expected a bare priority CASE, got:\n%s", single)
	}
}

// TestLatestMetricsQueryDedupesBySourcePriority exists because the latest value
// and the series drawn beside it used to be resolved differently: the series
// deduplicated by source priority while the latest row was picked by timestamp
// alone. A lower-priority device writing a minute later then decided both the
// value and the source name shown next to a sparkline computed from the other
// device.
func TestLatestMetricsQueryDedupesBySourcePriority(t *testing.T) {
	query := latestMetricsQuery([]string{"Oura", ""})

	checks := []string{
		// Step one: the newest timestamp per metric, straight off the index.
		"SELECT DISTINCT ON (metric_name) metric_name, time AS peak",
		// Step two: only the five minutes priority is defined over.
		"h.time > n.peak - interval '5 minutes'",
		// Priority decides within that window, recency breaks the tie.
		"WHEN source LIKE 'Oura%' THEN 1",
		"ORDER BY h.metric_name,",
		"h.time DESC",
	}

	for _, check := range checks {
		if !strings.Contains(query, check) {
			t.Errorf("latestMetricsQuery missing %q in:\n%s", check, query)
		}
	}
}

// TestLatestMetricsQueryDoesNotWindowTheWholeTable exists because the obvious
// way to apply source priority — ROW_NUMBER over every row, then DISTINCT ON —
// numbered 4.5 million rows to return seventeen and cost the front page five
// seconds. The shape, not the result, is what regresses.
func TestLatestMetricsQueryDoesNotWindowTheWholeTable(t *testing.T) {
	query := latestMetricsQuery([]string{"Oura", ""})

	if strings.Contains(query, "ROW_NUMBER") {
		t.Errorf("latestMetricsQuery numbers rows again:\n%s", query)
	}
	// Every scan of health_metrics has to be bounded by a time predicate.
	if strings.Contains(query, "PARTITION BY") {
		t.Errorf("latestMetricsQuery partitions again:\n%s", query)
	}
}

// TestLatestMetricsRecentQueryIsBoundedInTime exists because health_metrics is
// a hypertable: without a lower bound on time, TimescaleDB cannot exclude
// chunks and each per-metric lookup walks back through all of them. The bound
// has to sit inside the LATERAL subquery, where the chunk scan happens — not in
// the outer join.
func TestLatestMetricsRecentQueryIsBoundedInTime(t *testing.T) {
	since := time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC)
	query := latestMetricsForNamesRecentQuery([]string{"Oura", ""}, since)

	lateral := strings.Index(query, "CROSS JOIN LATERAL")
	closing := strings.Index(query[lateral:], ") l")
	if lateral < 0 || closing < 0 {
		t.Fatalf("unexpected query shape:\n%s", query)
	}
	inner := query[lateral : lateral+closing]

	// A literal, not a parameter: a bind parameter hides the value from the
	// planner, which then cannot exclude chunks and plans across all 514 of
	// them — 584ms of planning against 12ms, measured in production.
	if !strings.Contains(inner, "h.time >= TIMESTAMPTZ '2026-04-07") {
		t.Errorf("the lower bound is not an inlined literal inside the LATERAL:\n%s", query)
	}
	if !strings.Contains(inner, "LIMIT 1") {
		t.Errorf("expected a LIMIT 1 lookup per metric:\n%s", query)
	}
}

// TestDedupCTEMultiMetricRangeFiltersInsideTheCTE exists because the same
// filter one level out — in the caller's WHERE — makes Postgres number the
// user's whole history before narrowing to the window, which is why the front
// page took the same time for 30 days as for a year.
func TestDedupCTEMultiMetricRangeFiltersInsideTheCTE(t *testing.T) {
	cte := dedupCTEMultiMetricRange(map[string][]string{"heart_rate": {"Oura", ""}}, []string{"heart_rate"}, "$1", "$2,$3", "$4", "$5")

	openParen := strings.Index(cte, "(")
	closeParen := strings.LastIndex(cte, ")")
	if openParen < 0 || closeParen < 0 {
		t.Fatalf("unexpected CTE shape:\n%s", cte)
	}
	inner := cte[openParen:closeParen]

	for _, check := range []string{"time >= $4", "time < $5"} {
		if !strings.Contains(inner, check) {
			t.Errorf("range predicate %q is outside the CTE body:\n%s", check, cte)
		}
	}
}

// TestLatestMetricsQueryWithoutPrioritiesIsANoOp verifies the query still
// resolves when no priority is configured, rather than emitting an empty CASE.
func TestLatestMetricsQueryWithoutPrioritiesIsANoOp(t *testing.T) {
	query := latestMetricsQuery(nil)

	// sourcePriorityCaseSQL collapses to the constant 1, leaving the client rank
	// and then recency as the tiebreakers rather than emitting an empty CASE.
	if !strings.Contains(query, "ORDER BY h.metric_name, 1, "+clientRankSQL+", h.time DESC") {
		t.Errorf("expected the no-op ordering, got:\n%s", query)
	}
}

// TestDedupCTEMultiMetric verifies the multi-metric CTE partitions by both
// metric_name and time bucket, preventing cross-metric deduplication.
func TestDedupCTEMultiMetric(t *testing.T) {
	cte := dedupCTEMultiMetric(
		map[string][]string{"heart_rate": {"Oura", ""}, "body_mass": {"Oura", ""}},
		[]string{"heart_rate", "body_mass"}, "$1", "$2,$3")

	checks := []string{
		"WITH deduped AS",
		"PARTITION BY metric_name, time_bucket('5 minutes', time)",
		"user_id = $1",
		"metric_name IN ($2,$3)",
	}

	for _, check := range checks {
		if !strings.Contains(cte, check) {
			t.Errorf("dedupCTEMultiMetric missing %q in:\n%s", check, cte)
		}
	}
}
