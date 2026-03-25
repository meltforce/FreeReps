-- Per-user metric visibility configuration.
-- When a row exists, it overrides the default. Missing rows fall back to
-- a hardcoded default set in application code.
CREATE TABLE user_metric_visibility (
    user_id     INTEGER NOT NULL,
    metric_name TEXT    NOT NULL,
    visible     BOOLEAN NOT NULL DEFAULT TRUE,
    PRIMARY KEY (user_id, metric_name)
);
