-- Rows that differ only by client collide on the old key; keep the one whose
-- client sorts first ('' before 'freereps_ios' before 'hae').
DELETE FROM health_metrics a
USING health_metrics b
WHERE a.metric_name = b.metric_name AND a.source = b.source
  AND a.time = b.time AND a.user_id = b.user_id
  AND a.client > b.client;

DROP INDEX IF EXISTS idx_health_metrics_dedup_cover;
DROP INDEX IF EXISTS idx_health_metrics_dedup;
ALTER TABLE health_metrics DROP COLUMN client;

CREATE UNIQUE INDEX idx_health_metrics_dedup
    ON health_metrics (metric_name, source, time, user_id);
CREATE INDEX idx_health_metrics_dedup_cover
    ON health_metrics (user_id, metric_name, time DESC)
    INCLUDE (source, qty, avg_val, min_val, max_val);
