# Oura Ring Integration for FreeReps

## Context

You have an Oura Ring and want to pull its data into FreeReps. The Oura API v2 provides rich health data — including Oura-exclusive metrics (readiness, sleep score, stress, resilience, cardiovascular age, temperature deviation) that aren't available through HealthKit.

**The overlap problem**: Adding Oura as a second data source exposes a gap — FreeReps currently has no source awareness. HAE quantity metrics (HR, HRV, SpO2, steps) store `source = ""` with no device identification. Queries aggregate across all sources via AVG/SUM with no deduplication. This means without fixing source handling first, overlapping metrics would double-count (steps summed from both devices) or produce noisy averages.

The integration has two parts: **source awareness** (prerequisite) and **Oura sync** (the feature).

## Source Priority (Fixed 5-Minute Deduplication)

The ingest pathway determines the source — no iOS app changes needed:

| Data path | `source` value | Priority |
|---|---|---|
| Oura API → FreeReps (new) | `"Oura"` | Highest |
| HealthKit → iOS app / HAE → FreeReps | `""` (existing) | Lowest |

### How dedup works

Deduplication happens at query time using a **fixed 5-minute resolution**, independent of the display bucket size. This ensures consistent results at any zoom level.

Within each 5-minute window, if multiple sources have data, only the highest-priority source's samples are kept. The surviving samples are then aggregated into the requested display bucket (1 hour, 1 day, etc.).

```sql
-- CTE deduplicates at fixed 5-min granularity
WITH deduped AS (
  SELECT *,
    ROW_NUMBER() OVER (
      PARTITION BY time_bucket('5 minutes', time)
      ORDER BY CASE source WHEN 'Oura' THEN 1 ELSE 2 END
    ) AS rn
  FROM health_metrics
  WHERE metric_name = $2 AND time >= $3 AND time < $4 AND user_id = $5
)
-- Then aggregate to display bucket
SELECT time_bucket($1::interval, time) AS bucket,
       AVG(qty) AS avg_val, MIN(qty) AS min_val, MAX(qty) AS max_val, COUNT(*) AS count
FROM deduped WHERE rn = 1
GROUP BY bucket ORDER BY bucket
```

### Scope of query changes

- `GetTimeSeries()` — wrap in dedup CTE
- `GetCorrelation()` — same pattern
- `GetMetricStats()` — same pattern
- `GetDailySums()` — same pattern (critical for cumulative metrics like steps/calories)
- One helper function to build the source-priority `CASE WHEN` from config
- Config: `source_priority: ["Oura", ""]` (prefix matching, e.g. "Apple Watch" covers all models)
- Files: `server/internal/storage/health_metrics.go`, `server/internal/config/config.go`

---

## Oura Integration

### Approach: Server-Side Polling (No Webhooks)

- FreeReps is behind Tailscale — not publicly reachable for webhook callbacks
- Polling every 30 min uses ~10 requests/cycle vs 5000/5min rate limit
- Oura app must be opened to sync data anyway, so polling latency is fine

### Data Mapping

#### Oura-exclusive metrics → `health_metrics` (no overlap concern)

| Oura Data | FreeReps metric_name | Notes |
|---|---|---|
| Daily readiness score | `oura_readiness_score` | 0-100 |
| Daily sleep score | `oura_sleep_score` | 0-100 |
| Daily activity score | `oura_activity_score` | 0-100 |
| Temperature deviation | `oura_temperature_deviation` | Degrees from baseline |
| Stress high (seconds) | `oura_stress_high` | Seconds in high stress |
| Recovery high (seconds) | `oura_recovery_high` | Seconds in high recovery |
| Resilience level | `oura_resilience` | Encoded 1-5 |
| Cardiovascular age | `oura_cardiovascular_age` | Predicted vascular age |

#### Overlapping metrics → `health_metrics` (source priority handles dedup)

| Oura Data | FreeReps metric_name | Also from Apple Watch? |
|---|---|---|
| SpO2 average | `blood_oxygen_saturation` | Yes |
| VO2 max | `vo2_max` | Yes |
| Resting HR (from sleep) | `resting_heart_rate` | Yes |
| HRV (from sleep) | `heart_rate_variability` | Yes |
| Respiratory rate | `respiratory_rate` | Yes |
| Heart rate time-series | `heart_rate` | Yes |
| Steps | `step_count` | Yes |
| Active calories | `active_energy` | Yes |

All stored with `source = "Oura"`. Source priority config ensures Oura wins over Apple Watch when both report the same metric in the same time bucket.

#### Sleep → `sleep_sessions` + `sleep_stages`

- `SleepModel` → `SleepSessionRow` (durations seconds → hours, "light" → "Core")
- `sleep_phase_5_min` string → `SleepStageRow` entries (each char = 5min: 1=Deep, 2=Light, 3=REM, 4=Awake)
- Sleep sessions table has `ON CONFLICT (user_id, date) DO NOTHING` — first source to insert wins. With Oura syncing more frequently, it will typically win.

#### Workouts → `workouts` table

- Deterministic UUID from Oura workout ID
- Maps activity type, intensity, duration, calories, distance

### Package Structure

```
server/internal/oura/
    client.go       -- Oura API v2 HTTP client
    models.go       -- API response structs
    mapper.go       -- Oura data → FreeReps rows
    sync.go         -- Polling orchestrator
    token.go        -- OAuth2 token storage + refresh
    *_test.go       -- Tests for each
```

Separate from `internal/ingest/` — this is a pull integration with its own lifecycle (polling, token management), not a push provider.

### OAuth2 Flow

1. Register Oura API Application at `cloud.ouraring.com/oauth/applications`
2. Configure `client_id` and `client_secret` in `config.yaml`
3. "Authorize Oura" in FreeReps settings → Oura OAuth2 → callback to `https://freereps.leo-royal.ts.net/oura/callback` (browser-side redirect, not server-to-server — works behind Tailscale)
4. Tokens stored in `oura_tokens` DB table, auto-refreshed before expiry

### Database Migration

New migration adds:
- `oura_tokens` table (access_token, refresh_token, expires_at)
- `oura_sync_state` table (per-data-type last_sync tracking for incremental fetch)
- New `metric_allowlist` entries for Oura-specific metrics

### Config

```yaml
oura:
  enabled: false
  client_id: ""
  client_secret: ""
  sync_interval: "30m"
  backfill_days: 90

source_priority:    # applies globally, not just Oura
  - "Oura"
  - "Apple Watch"
  - ""
```

### New HTTP Endpoints

| Route | Purpose |
|---|---|
| `GET /api/v1/oura/status` | Sync status, last sync times, token expiry |
| `POST /api/v1/oura/authorize` | Exchange OAuth2 code for tokens |
| `POST /api/v1/oura/sync` | Trigger immediate manual sync |
| `DELETE /api/v1/oura/disconnect` | Remove tokens, stop syncing |
| `GET /oura/callback` | OAuth2 redirect handler |

---

## Implementation Phases

### Phase 1: Source Priority + Oura Plumbing
- Add `source_priority` + `OuraConfig` to config (`server/internal/config/config.go`)
- Add dedup CTE helper function + modify `GetTimeSeries()`, `GetCorrelation()`, `GetMetricStats()`, `GetDailySums()` with fixed 5-min source dedup (`server/internal/storage/health_metrics.go`)
- Migration `000010_oura.up.sql`: `oura_tokens` table, `oura_sync_state` table, new allowlist entries
- Storage methods for `oura_tokens` and `oura_sync_state`
- Tests for source-priority query logic + new storage methods

### Phase 2: Oura API Client
- Response structs from OpenAPI spec (`models.go`)
- HTTP client wrapping all endpoints (`client.go`)
- OAuth2 token exchange + refresh (`token.go`)
- Tests with httptest

### Phase 3: Data Mapping
- Oura → FreeReps row conversions (`mapper.go`)
- Sleep phase string parsing
- Tests (especially sleep mapping edge cases)

### Phase 4: Sync Orchestrator
- Polling loop with configurable interval (`sync.go`)
- Per-data-type incremental sync using `oura_sync_state`
- Initial backfill logic
- Wire into `main.go`
- Tests

### Phase 5: HTTP Handlers + Frontend
- Oura management handlers (status, authorize, sync, disconnect)
- OAuth2 callback handler
- Frontend settings panel (connection status, authorize button, manual sync trigger)

## Key Files to Modify

- `server/internal/storage/health_metrics.go` — source-priority query logic in `GetTimeSeries()`
- `server/internal/config/config.go` — add `OuraConfig` + `SourcePriority`
- `server/cmd/freereps/main.go` — start Oura sync goroutine
- `server/internal/server/server.go` — register Oura routes
- `server/migrations/` — new migration for oura_tokens, oura_sync_state, allowlist entries

## New Files

- `server/internal/oura/client.go` — Oura API v2 HTTP client
- `server/internal/oura/models.go` — API response structs
- `server/internal/oura/mapper.go` — Oura data → FreeReps rows
- `server/internal/oura/sync.go` — Polling orchestrator
- `server/internal/oura/token.go` — OAuth2 token storage + refresh

## Verification

1. `cd server && go vet ./...` after each phase
2. `go test ./...` for unit tests
3. After Phase 1: test source-priority queries with mock data — insert rows with source="" and source="Oura" for same metric/bucket, verify query returns Oura's values
4. After Phase 4: configure Oura OAuth2, authorize, trigger sync, verify data in dashboard
5. Overlap test: confirm that when Oura and HealthKit both report HR in the same bucket, dashboard shows Oura's values
6. Cumulative test: confirm steps/calories aren't double-counted across sources
