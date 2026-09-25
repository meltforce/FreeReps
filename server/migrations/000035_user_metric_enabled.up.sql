-- Per-user ingest enablement. A row overrides the default, which is enabled.
-- metric_allowlist.enabled stays the server-wide gate; a metric is accepted
-- for a user only when both allow it.
CREATE TABLE user_metric_enabled (
    user_id     INTEGER NOT NULL,
    metric_name TEXT    NOT NULL,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    PRIMARY KEY (user_id, metric_name)
);
