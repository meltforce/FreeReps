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
    ON health_metrics (metric_name, source, time, user_id);
```

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

Alpha Progression set/rep/weight data.

```sql
CREATE TABLE workout_sets (
    id                  BIGSERIAL   PRIMARY KEY,
    user_id             INTEGER     NOT NULL DEFAULT 1,
    session_name        TEXT        NOT NULL,
    session_date        TIMESTAMPTZ NOT NULL,
    session_duration    TEXT,
    exercise_number     INTEGER     NOT NULL,
    exercise_name       TEXT        NOT NULL,
    equipment           TEXT,
    target_reps         INTEGER,
    is_warmup           BOOLEAN     NOT NULL DEFAULT FALSE,
    set_number          INTEGER     NOT NULL,
    weight_kg           DOUBLE PRECISION,
    is_bodyweight_plus  BOOLEAN     NOT NULL DEFAULT FALSE,
    reps                INTEGER     NOT NULL,
    rir                 DOUBLE PRECISION,
    UNIQUE (user_id, session_date, exercise_number, set_number, is_warmup)
);
```

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

## Deduplication Strategy

All tables use `INSERT ... ON CONFLICT DO NOTHING`.
Unique constraints on natural keys prevent duplicate data from repeated syncs.

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
