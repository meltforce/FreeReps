# Database Schema Spec

## Overview

PostgreSQL 16 + TimescaleDB extension.
All timestamps as `TIMESTAMPTZ`.
All tables have `user_id DEFAULT 1` (multi-user ready, single-user for v1).

## Tables

### `health_metrics` (Hypertable)

Single wide table for all scalar health metrics.

```sql
CREATE TABLE health_metrics (
    time        TIMESTAMPTZ NOT NULL,
    user_id     INTEGER     NOT NULL DEFAULT 1,
    metric_name TEXT        NOT NULL,
    source      TEXT        NOT NULL DEFAULT '',
    client      TEXT        NOT NULL DEFAULT '',  -- 'freereps_ios', 'hae' or ''
    units       TEXT        NOT NULL DEFAULT '',
    qty         DOUBLE PRECISION,
    min_val     DOUBLE PRECISION,
    avg_val     DOUBLE PRECISION,
    max_val     DOUBLE PRECISION,
    systolic    DOUBLE PRECISION,
    diastolic   DOUBLE PRECISION
);

SELECT create_hypertable('health_metrics', 'time');

CREATE UNIQUE INDEX idx_health_metrics_dedup
    ON health_metrics (metric_name, source, client, time, user_id);
```

`client` names the ingest client of an Apple Health row: `freereps_ios` for
the iOS app, `hae` for every other Health Auto Export path (REST automation, TCP
import, uploaded export), `''` for Oura, Withings, derived rows and every row
stored before migration `000036`. Both Apple Health clients write
`source = ''`, so `client` is what tells their rows apart. Queries resolve one
winner per partition by source priority first and then by client —
`freereps_ios` before `hae` before `''` (`clientRankSQL` in
`internal/storage/health_metrics.go`) — so where both clients delivered a day,
only the app's rows count.

**Metric shapes:**
- Standard (qty): `resting_heart_rate`, `heart_rate_variability`, `blood_oxygen_saturation`, `respiratory_rate`, `vo2_max`, `weight_body_mass`, `body_fat_percentage`, `active_energy`, `basal_energy_burned`, `apple_exercise_time`, `apple_sleeping_wrist_temperature`
- Min/Avg/Max: `heart_rate`
- Systolic/Diastolic: `blood_pressure`

### `sleep_sessions` (Regular)

Aggregated nightly sleep summaries.

```sql
CREATE TABLE sleep_sessions (
    id              BIGSERIAL   PRIMARY KEY,
    user_id         INTEGER     NOT NULL DEFAULT 1,
    date            DATE        NOT NULL,
    total_sleep     DOUBLE PRECISION,
    asleep          DOUBLE PRECISION,
    core            DOUBLE PRECISION,
    deep            DOUBLE PRECISION,
    rem             DOUBLE PRECISION,
    in_bed          DOUBLE PRECISION,
    sleep_start     TIMESTAMPTZ,
    sleep_end       TIMESTAMPTZ,
    in_bed_start    TIMESTAMPTZ,
    in_bed_end      TIMESTAMPTZ,
    UNIQUE (user_id, date)
);
```

### `sleep_stages` (Hypertable)

Individual sleep stage segments for hypnogram.

```sql
CREATE TABLE sleep_stages (
    start_time  TIMESTAMPTZ NOT NULL,
    end_time    TIMESTAMPTZ NOT NULL,
    user_id     INTEGER     NOT NULL DEFAULT 1,
    stage       TEXT        NOT NULL,
    duration_hr DOUBLE PRECISION,
    source      TEXT        NOT NULL DEFAULT ''
);

SELECT create_hypertable('sleep_stages', 'start_time');

CREATE UNIQUE INDEX idx_sleep_stages_dedup
    ON sleep_stages (start_time, end_time, stage, user_id);
```

Stage values: `Awake`, `Asleep`, `In Bed`, `Core`, `REM`, `Deep`, `Unspecified`

### `workouts` (Regular)

Workout sessions with summary data.

```sql
CREATE TABLE workouts (
    id                      UUID PRIMARY KEY,
    user_id                 INTEGER     NOT NULL DEFAULT 1,
    name                    TEXT        NOT NULL,
    start_time              TIMESTAMPTZ NOT NULL,
    end_time                TIMESTAMPTZ NOT NULL,
    duration_sec            DOUBLE PRECISION,
    location                TEXT,
    is_indoor               BOOLEAN,
    active_energy_burned    DOUBLE PRECISION,
    active_energy_units     TEXT,
    total_energy            DOUBLE PRECISION,
    total_energy_units      TEXT,
    distance                DOUBLE PRECISION,
    distance_units          TEXT,
    avg_heart_rate          DOUBLE PRECISION,
    max_heart_rate          DOUBLE PRECISION,
    min_heart_rate          DOUBLE PRECISION,
    elevation_up            DOUBLE PRECISION,
    elevation_down          DOUBLE PRECISION,
    raw_json                JSONB,
    UNIQUE (user_id, id)
);
```

`raw_json` stores the full original workout JSON for fields we don't explicitly model.

### `workout_heart_rate` (Hypertable)

HR time-series during workouts.

```sql
CREATE TABLE workout_heart_rate (
    time        TIMESTAMPTZ NOT NULL,
    workout_id  UUID        NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    user_id     INTEGER     NOT NULL DEFAULT 1,
    min_bpm     DOUBLE PRECISION,
    avg_bpm     DOUBLE PRECISION,
    max_bpm     DOUBLE PRECISION,
    source      TEXT        NOT NULL DEFAULT ''
);

SELECT create_hypertable('workout_heart_rate', 'time');

CREATE UNIQUE INDEX idx_workout_hr_dedup
    ON workout_heart_rate (time, workout_id, user_id);
```

### `workout_routes` (Hypertable)

GPS route points.

```sql
CREATE TABLE workout_routes (
    time                TIMESTAMPTZ     NOT NULL,
    workout_id          UUID            NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    user_id             INTEGER         NOT NULL DEFAULT 1,
    latitude            DOUBLE PRECISION NOT NULL,
    longitude           DOUBLE PRECISION NOT NULL,
    altitude            DOUBLE PRECISION,
    speed               DOUBLE PRECISION,
    course              DOUBLE PRECISION,
    horizontal_accuracy DOUBLE PRECISION,
    vertical_accuracy   DOUBLE PRECISION
);

SELECT create_hypertable('workout_routes', 'time');

CREATE UNIQUE INDEX idx_workout_routes_dedup
    ON workout_routes (time, workout_id, user_id);
```

### `workout_sets` (Regular)

Set/rep/weight data. Two sources write here: the Alpha Progression CSV import
and the Hevy sync.

```sql
CREATE TABLE workout_sets (
    id                   BIGSERIAL   PRIMARY KEY,
    user_id              INTEGER     NOT NULL DEFAULT 1,
    source               TEXT        NOT NULL DEFAULT '',
    external_id          TEXT        NOT NULL DEFAULT '',
    routine_id           TEXT        NOT NULL DEFAULT '',
    session_name         TEXT        NOT NULL,
    session_date         TIMESTAMPTZ NOT NULL,
    session_end          TIMESTAMPTZ,
    session_duration     TEXT,
    exercise_number      INTEGER     NOT NULL,
    exercise_name        TEXT        NOT NULL,
    exercise_template_id TEXT        NOT NULL DEFAULT '',
    exercise_notes       TEXT        NOT NULL DEFAULT '',
    equipment            TEXT,
    target_reps          INTEGER,
    is_warmup            BOOLEAN     NOT NULL DEFAULT FALSE,
    set_type             TEXT        NOT NULL DEFAULT 'normal',
    set_number           INTEGER     NOT NULL,
    superset_id          INTEGER,
    weight_kg            DOUBLE PRECISION,
    is_bodyweight_plus   BOOLEAN     NOT NULL DEFAULT FALSE,
    reps                 INTEGER     NOT NULL,
    rir                  DOUBLE PRECISION,
    rpe                  DOUBLE PRECISION,
    effort_rir           DOUBLE PRECISION
        GENERATED ALWAYS AS (COALESCE(NULLIF(rir, -1), 10 - rpe)) STORED,
    distance_m           DOUBLE PRECISION,
    duration_sec         DOUBLE PRECISION,
    custom_metric        DOUBLE PRECISION,
    CONSTRAINT workout_sets_source_natural_key
        UNIQUE (user_id, source, session_date, exercise_number, set_number, is_warmup)
);

CREATE INDEX idx_workout_sets_source_external ON workout_sets (source, external_id);
```

`source` is `Alpha Progression` or `Hevy`. Columns each source leaves empty:
Alpha has no `external_id`, `routine_id`, `exercise_template_id` or
`superset_id`; Hevy has no `equipment`, `target_reps` or `is_bodyweight_plus`,
because those are properties of the exercise template or the routine rather than
of a logged set.

`set_type` holds Hevy's raw value (`normal`, `warmup`, `failure`, `dropset`);
`is_warmup` stays the column every aggregate query filters on.

**Effort is stored on the scale the source used.** Alpha Progression writes
`rir` (Reps in Reserve) with `-1` for an unrated set; Hevy writes `rpe` (Rating
of Perceived Exertion) and leaves `rir` empty. Neither is converted at write
time — `DECISIONS.md` 2026-03-25 keeps source normalization out of the ingest
path.

`effort_rir` is generated by the database from whichever column is filled and is
what every aggregate query reads. A `NULL` there means the set carries no rating
from either source, which replaces the older `-1` sentinel check.

### `hevy_credentials` (Regular)

```sql
CREATE TABLE hevy_credentials (
    user_id    INTEGER     NOT NULL PRIMARY KEY REFERENCES users(id),
    api_key    TEXT        NOT NULL,
    sync_from  DATE        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

`sync_from` bounds the ingest: workouts that started before this date are
discarded, so an Alpha history later uploaded to Hevy cannot flow back and count
a second time.

### `hevy_sync_state` (Regular)

```sql
CREATE TABLE hevy_sync_state (
    user_id       INTEGER     NOT NULL PRIMARY KEY REFERENCES users(id),
    last_event_at TIMESTAMPTZ NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

`last_event_at` is the `since` parameter of the next event fetch, minus a one
hour overlap.

### `withings_tokens` (Regular)

```sql
CREATE TABLE withings_tokens (
    user_id          INTEGER     NOT NULL PRIMARY KEY,
    client_id        TEXT        NOT NULL DEFAULT '',
    client_secret    TEXT        NOT NULL DEFAULT '',
    access_token     TEXT        NOT NULL DEFAULT '',
    refresh_token    TEXT        NOT NULL DEFAULT '',
    token_type       TEXT        NOT NULL DEFAULT 'Bearer',
    expires_at       TIMESTAMPTZ NOT NULL DEFAULT '1970-01-01',
    withings_user_id TEXT        NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Every column defaults, so a row can hold credentials before authorization has
happened — that is the state between saving the client ID and completing the
consent flow.

`refresh_token` rotates on every refresh and the previous value stops working
within hours (`withings-api.md`). Writes go through an upsert, never a bare
`UPDATE`, so a missing row cannot turn the write into a silent no-op.

### `withings_sync_state` (Regular)

```sql
CREATE TABLE withings_sync_state (
    user_id     INTEGER NOT NULL,
    data_type   TEXT    NOT NULL,
    last_update BIGINT  NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, data_type)
);
```

`last_update` is the `updatetime` value the previous `getmeas` response reported,
passed back as `lastupdate`. It is Unix seconds rather than a date because the
API filters to the second, and it comes from the server rather than the local
clock so the delta window does not depend on clock skew.

### `metric_allowlist` (Regular)

Controls which metrics are accepted during ingest.

```sql
CREATE TABLE metric_allowlist (
    metric_name TEXT PRIMARY KEY,
    category    TEXT NOT NULL,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE
);
```

**V1 seed data:**

| metric_name | category |
|-------------|----------|
| heart_rate | cardiovascular |
| resting_heart_rate | cardiovascular |
| heart_rate_variability | cardiovascular |
| blood_oxygen_saturation | cardiovascular |
| respiratory_rate | cardiovascular |
| vo2_max | cardiovascular |
| sleep_analysis | sleep |
| apple_sleeping_wrist_temperature | sleep |
| weight_body_mass | body |
| body_fat_percentage | body |
| active_energy | activity |
| basal_energy_burned | activity |
| apple_exercise_time | activity |

`enabled` is the server-wide gate. A metric is stored for a user only when it is
enabled here and not disabled in `user_metric_enabled`.

### `user_metric_enabled` (Regular)

Per-user override of `metric_allowlist.enabled`; a missing row means enabled.
Set from the Ingest settings tab via `PUT /api/v1/metrics/enabled`, read by
`GET /api/v1/allowlist`, which returns `enabled` resolved for the calling user.
Disabling a metric rejects new rows from every client and keeps stored rows.

```sql
CREATE TABLE user_metric_enabled (
    user_id     INTEGER NOT NULL,
    metric_name TEXT    NOT NULL,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    PRIMARY KEY (user_id, metric_name)
);
```

## Deduplication Strategy

Unique constraints on natural keys prevent duplicate data from repeated syncs.

`health_metrics` upserts: a row whose key (`metric_name, source, client, time, user_id`)
exists replaces the stored values when they differ, and an identical row writes
nothing. A client that aggregates into buckets sends the current bucket while it
fills and again once it is complete; under `DO NOTHING` the partial value stayed
(see [`INCIDENTS.md`](../../INCIDENTS.md), 2026-09-25). Rows repeating a key
within one call keep the last occurrence. The count returned to the client as
`metrics_inserted` includes changed rows.

`activity_summaries` upserts on `(user_id, date)` the same way: a day's
summary grows until the day ends and is sent on every sync.

The other tables use `INSERT ... ON CONFLICT DO NOTHING`; their keys are the
UUIDs of HealthKit samples, which do not change once written.

## TimescaleDB Features Used

- `create_hypertable()` on time-series tables for automatic partitioning
- `time_bucket()` for aggregated queries (daily, hourly averages)
- Compression policies (future optimization)

## Indexes

Beyond the unique dedup indexes:

```sql
CREATE INDEX idx_health_metrics_name_time ON health_metrics (metric_name, time DESC);
CREATE INDEX idx_workouts_start ON workouts (start_time DESC);
CREATE INDEX idx_workout_sets_date ON workout_sets (session_date DESC);
```
