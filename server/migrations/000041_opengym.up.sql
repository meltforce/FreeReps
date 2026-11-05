-- openGym ingest: credentials, change check and per-workout reconciliation.
-- Wire format and design: server/specs/opengym-api.md; DECISIONS.md, 2026-11-05.

-- One row per user. The token is an openGym session token from a pairing of its
-- own: a full sign-in to the profile, including write access (openGym offers no
-- scoped token). openGym renews it through GET /api/me past half its lifetime;
-- the sync writes the renewed token back here.
--
-- sync_from is a day in the session timezone. A workout that starts before it is
-- not ingested, because openGym's history includes the workouts imported from
-- Hevy, which FreeReps already holds under source 'Hevy'.
CREATE TABLE opengym_credentials (
    user_id    INTEGER     NOT NULL PRIMARY KEY REFERENCES users(id),
    base_url   TEXT        NOT NULL,
    token      TEXT        NOT NULL,
    sync_from  DATE        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The (rev, wid) pair of the state the last error-free cycle read. A cycle whose
-- pair matches skips GET /api/data. wid changes when openGym's data directory is
-- restored from a backup, after which rev can repeat.
CREATE TABLE opengym_sync_state (
    user_id    INTEGER     NOT NULL PRIMARY KEY REFERENCES users(id),
    rev        BIGINT      NOT NULL,
    wid        TEXT        NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One row per stored openGym workout: a hash of the rows the mapper produced for
-- it. A cycle rewrites a workout only when the hash changes, so a write to the
-- profile that does not touch a workout (a weigh-in, a setting) stores nothing
-- and the stale alert keeps measuring new training. A stored workout missing from
-- the state was deleted in openGym and its sets are removed.
CREATE TABLE opengym_workouts (
    user_id      INTEGER     NOT NULL REFERENCES users(id),
    workout_id   TEXT        NOT NULL,
    content_hash TEXT        NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, workout_id)
);
