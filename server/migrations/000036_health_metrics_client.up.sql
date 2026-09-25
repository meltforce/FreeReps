-- Which client delivered a row: 'freereps_ios' (the iOS app), 'hae' (Health Auto
-- Export) or '' (every other path, and rows stored before this column existed).
-- Both Apple Health clients write source = '', so without it their rows could not
-- be told apart: the app's hourly sums and Health Auto Export's minute rows were
-- summed together, and an hourly row and a minute row at the full hour shared one
-- key and overwrote each other.
ALTER TABLE health_metrics ADD COLUMN client TEXT NOT NULL DEFAULT '';

DROP INDEX IF EXISTS idx_health_metrics_dedup;
CREATE UNIQUE INDEX idx_health_metrics_dedup
    ON health_metrics (metric_name, source, client, time, user_id);

-- The dedup CTE now ranks by client as well; keep it an index-only scan.
DROP INDEX IF EXISTS idx_health_metrics_dedup_cover;
CREATE INDEX idx_health_metrics_dedup_cover
    ON health_metrics (user_id, metric_name, time DESC)
    INCLUDE (source, client, qty, avg_val, min_val, max_val);
