-- Covering index for the source-priority dedup CTE.
-- The dedup query filters by (user_id, metric_name, time range) then
-- partitions by time_bucket and orders by source. This index covers all
-- those columns, avoiding table lookups during the ROW_NUMBER() scan.
CREATE INDEX IF NOT EXISTS idx_health_metrics_dedup_cover
    ON health_metrics (user_id, metric_name, time DESC)
    INCLUDE (source, qty, avg_val, min_val, max_val);
