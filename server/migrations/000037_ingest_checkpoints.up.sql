-- Where each Apple Health client's delivered data ends, per data domain and, for
-- metrics, per metric name. The iOS app reads it after a reinstall, when its
-- local sync state is empty, to start its backfill there instead of at the
-- configured backfill window.
--
-- A row is advanced only after the insert it describes returned without error,
-- and newest_sample only moves forward, so it never names data the server does
-- not hold. A checkpoint that trails the data costs one redundant fetch; one
-- ahead of the data skips rows permanently.
CREATE TABLE ingest_checkpoints (
    user_id        INTEGER     NOT NULL,
    client         TEXT        NOT NULL,              -- health_metrics.client: 'freereps_ios' or 'hae'
    domain         TEXT        NOT NULL,              -- metrics, workouts, workout_routes, activity_summaries, state_of_mind, category_samples
    item           TEXT        NOT NULL DEFAULT '',   -- metric name for domain 'metrics', '' otherwise
    newest_sample  TIMESTAMPTZ NOT NULL,
    last_import_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, client, domain, item)
);
