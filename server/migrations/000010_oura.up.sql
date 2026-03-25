-- Oura Ring integration: token storage, sync state, and metric allowlist entries.

-- OAuth2 token storage for Oura API (one token per user).
CREATE TABLE oura_tokens (
    user_id       INTEGER     NOT NULL PRIMARY KEY,
    access_token  TEXT        NOT NULL,
    refresh_token TEXT        NOT NULL,
    token_type    TEXT        NOT NULL DEFAULT 'Bearer',
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Per-data-type incremental sync tracking.
CREATE TABLE oura_sync_state (
    user_id    INTEGER NOT NULL,
    data_type  TEXT    NOT NULL,
    last_sync  DATE    NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, data_type)
);

-- Oura-exclusive metrics in the allowlist.
INSERT INTO metric_allowlist (metric_name, category) VALUES
    ('oura_readiness_score',       'oura'),
    ('oura_sleep_score',           'oura'),
    ('oura_activity_score',        'oura'),
    ('oura_temperature_deviation', 'oura'),
    ('oura_stress_high',           'oura'),
    ('oura_recovery_high',         'oura'),
    ('oura_resilience',            'oura'),
    ('oura_cardiovascular_age',    'oura')
ON CONFLICT (metric_name) DO NOTHING;
