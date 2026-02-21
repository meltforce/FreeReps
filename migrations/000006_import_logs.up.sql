CREATE TABLE import_logs (
    id              BIGSERIAL PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    source          TEXT NOT NULL,         -- hae_rest, hae_tcp, alpha
    status          TEXT NOT NULL,         -- running, success, error
    metrics_received    INTEGER NOT NULL DEFAULT 0,
    metrics_inserted    BIGINT  NOT NULL DEFAULT 0,
    workouts_received   INTEGER NOT NULL DEFAULT 0,
    workouts_inserted   INTEGER NOT NULL DEFAULT 0,
    sleep_sessions      INTEGER NOT NULL DEFAULT 0,
    sets_inserted       BIGINT  NOT NULL DEFAULT 0,
    duration_ms         INTEGER,
    error_message       TEXT,
    metadata            JSONB
);

CREATE INDEX idx_import_logs_user_created ON import_logs (user_id, created_at DESC);
