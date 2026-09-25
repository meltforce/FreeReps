package storage

import (
	"context"
	"fmt"
	"math"
	"sort"
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

// dedupBucket is the window within which competing sources are compared for a
// non-cumulative metric.
const dedupBucket = "time_bucket('5 minutes', time)"

// quoteLiteral renders a metric name as a SQL string literal. Metric names come
// from the allowlist rather than from user input, but the queries below build
// IN clauses as text, so the escaping belongs here rather than at each site.
func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// sourcePartition returns the window within which exactly one source wins.
//
// Cumulative metrics resolve the source per day. Oura reports a day's steps as
// a single row while Apple Health reports hourly blocks; choosing per 5-minute
// window keeps the Oura total in its own window and adds the Apple blocks
// around it. On 2026-09-18 that produced 32361 steps where Apple Health had
// recorded 16652.
//
// Every other metric resolves per 5-minute window, because a source that stops
// reporting for part of a day must not remove that part. On the same day Apple
// Health held 54 windows in which Oura had no row at all, covering the morning
// strength session.
func sourcePartition(metricName string) string {
	if cumulativeMetrics[metricName] {
		return "time_bucket('1 day', time)"
	}
	return dedupBucket
}

// multiMetricPartition is sourcePartition for a query spanning several metrics.
// The branch is decided per group because metric_name is in the partition.
func multiMetricPartition(metricNames []string) string {
	var cumulative []string
	for _, name := range metricNames {
		if cumulativeMetrics[name] {
			cumulative = append(cumulative, quoteLiteral(name))
		}
	}
	if len(cumulative) == 0 {
		return "metric_name, " + dedupBucket
	}
	sort.Strings(cumulative)
	return fmt.Sprintf(
		"metric_name, CASE WHEN metric_name IN (%s) THEN time_bucket('1 day', time) ELSE %s END",
		strings.Join(cumulative, ","), dedupBucket)
}

// clientRankSQL orders the clients of one source: where the iOS app and Health
// Auto Export both delivered Apple Health data for a window, the app wins, and
// both win over rows stored before the client column existed. Both clients
// write source = '', so without this rank their rows were summed together.
const clientRankSQL = "CASE client WHEN 'freereps_ios' THEN 0 WHEN 'hae' THEN 1 ELSE 2 END"

// winningSourceRN marks every row of the highest-priority source in its
// partition with rn = 1, so callers keep filtering on "WHERE rn = 1". Within
// one source the client ranks next (clientRankSQL); source and client together
// are what wins.
//
// The predicate now removes competing sources rather than competing samples of
// the same source. ROW_NUMBER() kept exactly one row per window, which turned a
// sum over per-second samples into a sum over one of them, and made a bucket's
// average depend on which sample happened to sort first.
func winningSourceRN(priorityExpr, partition string) string {
	return fmt.Sprintf(
		`CASE WHEN (source, client) = (
				FIRST_VALUE(source) OVER (PARTITION BY %[1]s ORDER BY %[2]s, %[3]s, source, client),
				FIRST_VALUE(client) OVER (PARTITION BY %[1]s ORDER BY %[2]s, %[3]s, source, client)
			) THEN 1 ELSE 2 END AS rn`, partition, priorityExpr, clientRankSQL)
}

// perMetricPriorityExpr nests each metric's category priority inside a CASE over
// metric_name. A multi-metric query used to resolve every metric with the
// user's _default priority, which is why the front page and the metric page
// reported different step counts for the same day. Metrics sharing a priority
// list share a branch.
func perMetricPriorityExpr(priorityByMetric map[string][]string) string {
	byExpr := make(map[string][]string, len(priorityByMetric))
	for metric, priorities := range priorityByMetric {
		expr := sourcePriorityCaseSQL(priorities)
		byExpr[expr] = append(byExpr[expr], metric)
	}
	exprs := make([]string, 0, len(byExpr))
	for expr := range byExpr {
		exprs = append(exprs, expr)
	}
	sort.Strings(exprs)
	if len(exprs) == 1 {
		return exprs[0]
	}

	var b strings.Builder
	b.WriteString("CASE ")
	for _, expr := range exprs[:len(exprs)-1] {
		metrics := byExpr[expr]
		sort.Strings(metrics)
		quoted := make([]string, len(metrics))
		for i, m := range metrics {
			quoted[i] = quoteLiteral(m)
		}
		fmt.Fprintf(&b, "WHEN metric_name IN (%s) THEN (%s) ", strings.Join(quoted, ","), expr)
	}
	fmt.Fprintf(&b, "ELSE (%s) END", exprs[len(exprs)-1])
	return b.String()
}

// dedupCTE returns a WITH clause reducing health_metrics to the rows of one
// source per partition. Callers filter with "WHERE rn = 1".
func dedupCTE(priorities []string, metricName, metricParam, startParam, endParam, userIDParam string) string {
	rn := winningSourceRN(sourcePriorityCaseSQL(priorities), sourcePartition(metricName))
	return fmt.Sprintf(
		`WITH deduped AS (
			SELECT *, %s
			FROM health_metrics
			WHERE metric_name = %s AND time >= %s AND time < %s AND user_id = %s
		) `, rn, metricParam, startParam, endParam, userIDParam)
}

// dedupCTEMultiMetric returns a dedup CTE for queries that span multiple metrics
// (e.g. GetDailySums). Partitions by metric_name in addition to the time window.
func dedupCTEMultiMetric(priorityByMetric map[string][]string, metricNames []string, userIDParam, inClause string) string {
	rn := winningSourceRN(perMetricPriorityExpr(priorityByMetric), multiMetricPartition(metricNames))
	return fmt.Sprintf(
		`WITH deduped AS (
			SELECT *, %s
			FROM health_metrics
			WHERE user_id = %s AND metric_name IN (%s)
		) `, rn, userIDParam, inClause)
}

// dedupCTEMultiMetricRange is dedupCTEMultiMetric with the time range inside the
// CTE rather than in the caller's WHERE clause.
//
// That distinction is the whole cost of the query. Filtering outside makes
// Postgres number every row the user holds for those metrics before discarding
// all but the window — which is why the front page took the same five seconds
// whether it asked for 30 days or a year. Inside, the range joins the index
// condition on idx_health_metrics_dedup_cover.
func dedupCTEMultiMetricRange(priorityByMetric map[string][]string, metricNames []string, userIDParam, inClause, startParam, endParam string) string {
	rn := winningSourceRN(perMetricPriorityExpr(priorityByMetric), multiMetricPartition(metricNames))
	return fmt.Sprintf(
		`WITH deduped AS (
			SELECT *, %s
			FROM health_metrics
			WHERE user_id = %s AND metric_name IN (%s)
			  AND time >= %s AND time < %s
		) `, rn, userIDParam, inClause, startParam, endParam)
}

// cumulativeMetrics are metrics that should be summed (not averaged) when aggregating.
var cumulativeMetrics = map[string]bool{
	"active_energy":                 true,
	"basal_energy_burned":           true,
	"apple_exercise_time":           true,
	"step_count":                    true,
	"distance_walking_running":      true,
	"distance_cycling":              true,
	"distance_swimming":             true,
	"distance_wheelchair":           true,
	"flights_climbed":               true,
	"apple_move_time":               true,
	"apple_stand_time":              true,
	"push_count":                    true,
	"swimming_stroke_count":         true,
	"distance_downhill_snow_sports": true,
	// Daily training volume: a week's figure is the sum of its days, not their
	// average.
	TrainingTonnageMetric: true,
}

// maxRowsPerBatch is the PostgreSQL extended protocol parameter limit (65535)
// divided by 13 parameters per row, with headroom.
const maxRowsPerBatch = 5000

// InsertHealthMetrics batch-upserts health metric rows and returns the number of rows
// inserted or changed. A row whose key (metric_name, source, time, user_id) already
// exists replaces the stored values when they differ: a client that aggregates into
// buckets sends the bucket again once more samples have arrived, and the first,
// partial delivery must not win. An identical re-delivery writes nothing and is not
// counted. Rows repeating a key within one call keep the last occurrence.
func (db *DB) InsertHealthMetrics(ctx context.Context, rows []models.HealthMetricRow) (int64, error) {
	rows = dedupeHealthMetricRows(rows)
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
	query := `INSERT INTO health_metrics (time, user_id, metric_name, source, client, units, qty, min_val, avg_val, max_val, systolic, diastolic, source_uuid)
VALUES `
	args := make([]any, 0, len(rows)*13)
	valueStrings := make([]string, 0, len(rows))

	for i, r := range rows {
		base := i * 13
		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9, base+10, base+11, base+12, base+13,
		))
		args = append(args, r.Time, r.UserID, r.MetricName, r.Source, r.Client, r.Units,
			r.Qty, r.MinVal, r.AvgVal, r.MaxVal, r.Systolic, r.Diastolic, r.SourceUUID)
	}

	query += strings.Join(valueStrings, ",") + `
ON CONFLICT (metric_name, source, client, time, user_id) DO UPDATE SET
	units = EXCLUDED.units, qty = EXCLUDED.qty,
	min_val = EXCLUDED.min_val, avg_val = EXCLUDED.avg_val, max_val = EXCLUDED.max_val,
	systolic = EXCLUDED.systolic, diastolic = EXCLUDED.diastolic, source_uuid = EXCLUDED.source_uuid
WHERE (health_metrics.units, health_metrics.qty, health_metrics.min_val, health_metrics.avg_val,
       health_metrics.max_val, health_metrics.systolic, health_metrics.diastolic, health_metrics.source_uuid)
	IS DISTINCT FROM
      (EXCLUDED.units, EXCLUDED.qty, EXCLUDED.min_val, EXCLUDED.avg_val,
       EXCLUDED.max_val, EXCLUDED.systolic, EXCLUDED.diastolic, EXCLUDED.source_uuid)`

	tag, err := db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("inserting health metrics: %w", err)
	}
	return tag.RowsAffected(), nil
}

// dedupeHealthMetricRows drops earlier rows that share the conflict key with a later
// one. ON CONFLICT DO UPDATE fails when one statement touches the same row twice.
func dedupeHealthMetricRows(rows []models.HealthMetricRow) []models.HealthMetricRow {
	type key struct {
		metric, source, client string
		time                   time.Time
		user                   int
	}
	last := make(map[key]int, len(rows))
	for i, r := range rows {
		last[key{r.MetricName, r.Source, r.Client, r.Time.UTC(), r.UserID}] = i
	}
	if len(last) == len(rows) {
		return rows
	}
	out := make([]models.HealthMetricRow, 0, len(last))
	for i, r := range rows {
		if last[key{r.MetricName, r.Source, r.Client, r.Time.UTC(), r.UserID}] == i {
			out = append(out, r)
		}
	}
	return out
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

// GetLatestMetrics returns the most recent data point for each metric, resolved
// against the user's source priority.
//
// The priority applies within a 5-minute bucket, as everywhere else: the newest
// bucket wins, and inside it the highest-priority source. Ordering by time alone
// would let a lower-priority device that wrote a minute later decide both the
// value and the source name shown beside it — while the series next to it, which
// does dedupe by priority, came from the other device.
func (db *DB) GetLatestMetrics(ctx context.Context, userID int) ([]models.HealthMetricRow, error) {
	return db.latestMetrics(ctx, userID, nil)
}

// recentLookupDays bounds the first pass of the latest-value lookup.
//
// health_metrics is a hypertable, and without a time predicate TimescaleDB
// cannot exclude chunks — every per-metric lookup walks back through all of
// them until it finds a row, which costs the same whether the reading is from
// this morning or two years ago. Nearly every metric has something within this
// window, so the bounded pass answers almost all of them and the unbounded
// second pass runs for the few stragglers.
const recentLookupDays = 120

// GetLatestMetricsFor is GetLatestMetrics restricted to named metrics.
//
// Two passes: a bounded one that TimescaleDB can satisfy from recent chunks,
// then an unbounded one for whichever names it did not answer — a metric like
// body weight, last recorded months ago, still reports its value.
func (db *DB) GetLatestMetricsFor(ctx context.Context, userID int, names []string) ([]models.HealthMetricRow, error) {
	if len(names) == 0 {
		return nil, nil
	}

	priorities := db.ResolveSourcePriority(ctx, userID, "_default")
	since := time.Now().AddDate(0, 0, -recentLookupDays)

	recent, err := db.queryLatest(ctx, latestMetricsForNamesRecentQuery(priorities, since), userID, names)
	if err != nil {
		return nil, err
	}

	found := make(map[string]bool, len(recent))
	for _, row := range recent {
		found[row.MetricName] = true
	}
	var stale []string
	for _, name := range names {
		if !found[name] {
			stale = append(stale, name)
		}
	}
	if len(stale) == 0 {
		return recent, nil
	}

	older, err := db.queryLatest(ctx, latestMetricsForNamesQuery(priorities), userID, stale)
	if err != nil {
		return nil, err
	}
	return append(recent, older...), nil
}

func (db *DB) latestMetrics(ctx context.Context, userID int, names []string) ([]models.HealthMetricRow, error) {
	priorities := db.ResolveSourcePriority(ctx, userID, "_default")
	if names == nil {
		return db.queryLatest(ctx, latestMetricsQuery(priorities), userID)
	}
	return db.queryLatest(ctx, latestMetricsForNamesQuery(priorities), userID, names)
}

func (db *DB) queryLatest(ctx context.Context, query string, args ...any) ([]models.HealthMetricRow, error) {
	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying latest metrics: %w", err)
	}
	defer rows.Close()

	return scanHealthMetricRows(rows)
}

// latestMetricsQuery builds the deduplicated latest-per-metric query. Split out
// so the priority ordering can be asserted without a database.
//
// Two steps, both index-driven. The first walks idx_health_metrics_dedup_cover
// backwards to find the newest timestamp per metric — one row per metric, no
// scan. The second reopens only the five minutes before each of those, which is
// the window source priority is defined over.
//
// The obvious form — ROW_NUMBER over every row, then DISTINCT ON — is what made
// the front page take five seconds: it numbered 4.5 million rows to return
// seventeen.
func latestMetricsQuery(priorities []string) string {
	return fmt.Sprintf(
		`WITH newest AS (
			SELECT DISTINCT ON (metric_name) metric_name, time AS peak
			FROM health_metrics
			WHERE user_id = $1
			ORDER BY metric_name, time DESC
		)
		SELECT DISTINCT ON (h.metric_name)
		       h.time, h.user_id, h.metric_name, h.source, h.units,
		       h.qty, h.min_val, h.avg_val, h.max_val,
		       h.systolic, h.diastolic, h.source_uuid
		 FROM newest n
		 JOIN health_metrics h
		   ON h.user_id = $1
		  AND h.metric_name = n.metric_name
		  AND h.time > n.peak - interval '5 minutes'
		  AND h.time <= n.peak
		 ORDER BY h.metric_name, %s, %s, h.time DESC`, sourcePriorityCaseSQL(priorities), clientRankSQL)
}

// latestMetricsForNamesRecentQuery is latestMetricsForNamesQuery with a lower
// time bound, which is what lets TimescaleDB skip old chunks. Metrics with
// nothing in the window return no row and are retried unbounded.
func latestMetricsForNamesRecentQuery(priorities []string, since time.Time) string {
	return fmt.Sprintf(
		`WITH newest AS (
			SELECT m.metric_name, l.time AS peak
			FROM unnest($2::text[]) AS m(metric_name)
			CROSS JOIN LATERAL (
				SELECT time FROM health_metrics h
				WHERE h.user_id = $1 AND h.metric_name = m.metric_name
				  AND h.time >= %s
				ORDER BY h.time DESC
				LIMIT 1
			) l
		)
		SELECT DISTINCT ON (h.metric_name)
		       h.time, h.user_id, h.metric_name, h.source, h.units,
		       h.qty, h.min_val, h.avg_val, h.max_val,
		       h.systolic, h.diastolic, h.source_uuid
		 FROM newest n
		 JOIN health_metrics h
		   ON h.user_id = $1
		  AND h.metric_name = n.metric_name
		  AND h.time > n.peak - interval '5 minutes'
		  AND h.time <= n.peak
		 ORDER BY h.metric_name, %s, %s, h.time DESC`,
		sqlTimestamp(since), sourcePriorityCaseSQL(priorities), clientRankSQL)
}

// latestMetricsForNamesQuery is latestMetricsQuery with the metric names given
// as a parameter, so each one becomes a bounded index lookup rather than a walk
// across every entry the user has.
func latestMetricsForNamesQuery(priorities []string) string {
	return fmt.Sprintf(
		`WITH newest AS (
			SELECT m.metric_name, l.time AS peak
			FROM unnest($2::text[]) AS m(metric_name)
			CROSS JOIN LATERAL (
				SELECT time FROM health_metrics h
				WHERE h.user_id = $1 AND h.metric_name = m.metric_name
				ORDER BY h.time DESC
				LIMIT 1
			) l
		)
		SELECT DISTINCT ON (h.metric_name)
		       h.time, h.user_id, h.metric_name, h.source, h.units,
		       h.qty, h.min_val, h.avg_val, h.max_val,
		       h.systolic, h.diastolic, h.source_uuid
		 FROM newest n
		 JOIN health_metrics h
		   ON h.user_id = $1
		  AND h.metric_name = n.metric_name
		  AND h.time > n.peak - interval '5 minutes'
		  AND h.time <= n.peak
		 ORDER BY h.metric_name, %s, %s, h.time DESC`, sourcePriorityCaseSQL(priorities), clientRankSQL)
}

// timeSeriesSelectSQL returns the aggregation applied to the deduped rows.
//
// A cumulative metric sums every sample of the winning source. Every other
// metric averages within each 5-minute window first and then across those
// windows, so a workout's per-second samples weigh the same as a quiet hour.
// On 2026-09-18 six workouts covered 16% of the day and held 77% of the heart
// rate rows; averaging the rows directly would let them carry the daily figure.
// MIN and MAX stay over all rows, so the range reports the real extremes rather
// than the extremes of the window averages.
func timeSeriesSelectSQL(metricName string) string {
	if cumulativeMetrics[metricName] {
		return `SELECT time_bucket($1::interval, time) AS bucket,
		        SUM(COALESCE(qty, avg_val)) AS avg_val,
		        MIN(COALESCE(qty, min_val)) AS min_val,
		        MAX(COALESCE(qty, max_val)) AS max_val,
		        COUNT(*) AS count
		 FROM deduped WHERE rn = 1
		 GROUP BY bucket
		 ORDER BY bucket ASC`
	}
	return fmt.Sprintf(`, windowed AS (
			SELECT time_bucket($1::interval, time) AS bucket,
			       %s AS sub,
			       AVG(COALESCE(qty, avg_val)) AS avg_val,
			       MIN(COALESCE(qty, min_val)) AS min_val,
			       MAX(COALESCE(qty, max_val)) AS max_val,
			       COUNT(*) AS count
			FROM deduped WHERE rn = 1
			GROUP BY bucket, sub
		)
		SELECT bucket,
		       AVG(avg_val) AS avg_val,
		       MIN(min_val) AS min_val,
		       MAX(max_val) AS max_val,
		       SUM(count)::bigint AS count
		 FROM windowed
		 GROUP BY bucket
		 ORDER BY bucket ASC`, dedupBucket)
}

// GetTimeSeries returns aggregated time-series data using time_bucket.
// bucketSize should be a PostgreSQL interval like '1 day', '1 hour'.
// See timeSeriesSelectSQL for the aggregate each metric class receives.
func (db *DB) GetTimeSeries(ctx context.Context, metricName string, start, end time.Time, bucketSize string, userID int) ([]TimeSeriesPoint, error) {
	priorities := db.ResolveSourcePriorityForMetric(ctx, userID, metricName)
	query := dedupCTE(priorities, metricName, "$2", "$3", "$4", "$5") + timeSeriesSelectSQL(metricName)
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

// resolvePrioritiesFor returns each metric's category priority. A query spanning
// several metrics used to resolve all of them with the user's _default
// priority, so a metric whose category carried a different order was read from
// the wrong source.
func (db *DB) resolvePrioritiesFor(ctx context.Context, userID int, metricNames []string) map[string][]string {
	byMetric := make(map[string][]string, len(metricNames))
	for _, name := range metricNames {
		byMetric[name] = db.ResolveSourcePriorityForMetric(ctx, userID, name)
	}
	return byMetric
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
	cte := dedupCTEMultiMetric(db.resolvePrioritiesFor(ctx, userID, metricNames), metricNames, "$1", inClause)

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

// buildMetricStatsQuery assembles the statistics query for one metric.
//
// Avg carries a sum for cumulative metrics. A per-sample average answers "what
// was the average block of steps", which is not a figure anyone asks for; the
// range total is. The JSON field keeps its name because the MCP tools ship the
// same struct, so the callers label it from is_cumulative instead.
func buildMetricStatsQuery(priorities []string, metricName, metricParam, startParam, endParam, userIDParam string) string {
	cte := dedupCTE(priorities, metricName, metricParam, startParam, endParam, userIDParam)
	if cumulativeMetrics[metricName] {
		return cte + `SELECT SUM(COALESCE(qty, avg_val)),
		        MIN(COALESCE(qty, min_val)),
		        MAX(COALESCE(qty, max_val)),
		        STDDEV_POP(COALESCE(qty, avg_val)),
		        COUNT(*)
		 FROM deduped WHERE rn = 1`
	}
	return cte + fmt.Sprintf(`, windowed AS (
			SELECT %s AS sub,
			       AVG(COALESCE(qty, avg_val)) AS avg_val,
			       MIN(COALESCE(qty, min_val)) AS min_val,
			       MAX(COALESCE(qty, max_val)) AS max_val,
			       COUNT(*) AS count
			FROM deduped WHERE rn = 1
			GROUP BY sub
		)
		SELECT AVG(avg_val),
		       MIN(min_val),
		       MAX(max_val),
		       STDDEV_POP(avg_val),
		       COALESCE(SUM(count), 0)::bigint
		 FROM windowed`, dedupBucket)
}

// GetMetricStats returns aggregate statistics for a metric over a time range.
func (db *DB) GetMetricStats(ctx context.Context, metricName string, start, end time.Time, userID int) (*MetricStats, error) {
	priorities := db.ResolveSourcePriorityForMetric(ctx, userID, metricName)
	query := buildMetricStatsQuery(priorities, metricName, "$1", "$2", "$3", "$4")
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

// correlationSeriesSQL returns one side of the correlation join, aggregated the
// same way the metric is aggregated everywhere else: a sum for cumulative
// metrics, otherwise the average of the 5-minute window averages.
func correlationSeriesSQL(metricName, from string) string {
	if cumulativeMetrics[metricName] {
		return fmt.Sprintf(`SELECT time_bucket($1::interval, time) AS bucket,
			       SUM(COALESCE(qty, avg_val)) AS val
			FROM %s WHERE rn = 1
			GROUP BY bucket`, from)
	}
	return fmt.Sprintf(`SELECT bucket, AVG(val) AS val FROM (
				SELECT time_bucket($1::interval, time) AS bucket,
				       %s AS sub,
				       AVG(COALESCE(qty, avg_val)) AS val
				FROM %s WHERE rn = 1
				GROUP BY bucket, sub
			) windowed
			GROUP BY bucket`, dedupBucket, from)
}

// GetCorrelation joins two metrics on time buckets and computes their Pearson
// correlation. Each metric is aggregated the way its class requires; see
// correlationSeriesSQL.
func (db *DB) GetCorrelation(ctx context.Context, xMetric, yMetric string, start, end time.Time, bucket string, userID int) (*CorrelationResult, error) {
	// Each side resolves with its own category priority. Applying the X
	// metric's priority to Y picked the wrong source whenever the two sat in
	// different categories, for example steps against heart rate.
	xPriorities := db.ResolveSourcePriorityForMetric(ctx, userID, xMetric)
	yPriorities := db.ResolveSourcePriorityForMetric(ctx, userID, yMetric)
	query := fmt.Sprintf(
		`WITH x_deduped AS (
			SELECT *, %s
			FROM health_metrics
			WHERE metric_name = $2 AND time >= $4 AND time < $5 AND user_id = $6
		), y_deduped AS (
			SELECT *, %s
			FROM health_metrics
			WHERE metric_name = $3 AND time >= $4 AND time < $5 AND user_id = $6
		), x AS (
			%s
		), y AS (
			%s
		)
		SELECT x.bucket, x.val, y.val
		FROM x JOIN y ON x.bucket = y.bucket
		ORDER BY x.bucket ASC`,
		winningSourceRN(sourcePriorityCaseSQL(xPriorities), sourcePartition(xMetric)),
		winningSourceRN(sourcePriorityCaseSQL(yPriorities), sourcePartition(yMetric)),
		correlationSeriesSQL(xMetric, "x_deduped"),
		correlationSeriesSQL(yMetric, "y_deduped"))
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
