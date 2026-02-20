# FreeReps Fix Plan

## Phase Status

| Phase | Status |
|-------|--------|
| Phase 1 — Data Foundation | Complete |
| Phase 1.5 — CLI File Import | Complete |
| Phase 2 — Visualization | Complete |
| Phase 3 — MCP Integration | Not started |
| Phase 4 — Polish | Not started |

## Phase 1 — Data Foundation (Complete)

- [x] Project scaffolding + Docker Compose
- [x] Config loading (YAML + env override)
- [x] Database schema + TimescaleDB migrations
- [x] Storage layer (health_metrics, sleep, workouts, workout_sets)
- [x] Domain models (HAE REST, HAE file, Alpha Progression)
- [x] HAE ingest provider (REST API compatible)
- [x] Alpha Progression CSV provider
- [x] HTTP server + middleware (auth, CORS, logging)
- [x] API endpoints (metrics, sleep, workouts, timeseries, stats, correlation)
- [x] Embedded frontend (SPA served from Go binary)

## Phase 1.5 — CLI File Import (Complete)

- [x] .hae file format models
- [x] LZFSE decompression
- [x] File importer core logic
- [x] Workout HR correlation
- [x] CLI entry point (`freereps-import`)
- [x] Tests (parser, importer)

## Phase 2 — Visualization (Complete)

- [x] Navigation + routing (React Router)
- [x] Dashboard home (daily overview + time-series)
- [x] Sleep view (hypnogram + metric cards + history chart)
- [x] Workout view (list + detail with HR zones, route map, Alpha sets)
- [x] Metrics deep dive (stats, moving average, normal range band)
- [x] Correlation explorer (scatter, overlay, saved views, Pearson r)
- [x] Error boundaries + code splitting
- [x] Bug fix rounds R1-R6 (cumulative metrics, sleep backfill, Alpha parser, list UX)

## Phase 3 — MCP Integration (Next)

- [ ] Add `mark3labs/mcp-go` dependency
- [ ] MCP server setup (stdio + SSE transport)
- [ ] Tool: `get_health_metrics` — retrieve metrics for time range
- [ ] Tool: `get_workouts` — query workouts with filters
- [ ] Tool: `get_sleep_data` — sleep data with stages
- [ ] Tool: `get_metric_stats` — avg/min/max/stddev/trend
- [ ] Tool: `get_correlation` — Pearson r between two metrics
- [ ] Tool: `compare_periods` — compare two time ranges
- [ ] Tool: `list_available_metrics` — available metrics + data range
- [ ] Tool: `query_raw` — flexible query with filters
- [ ] Resource: `daily_summary`
- [ ] Resource: `recent_workouts`
- [ ] Resource: `metric_catalog`

## Phase 4 — Polish

- [ ] Trend views (multi-metric long-term analysis)
- [ ] Settings UI (metric enable/disable, personal parameters)
- [ ] Saveable dashboard layouts (beyond correlation saved views)
- [ ] Responsive refinement

## Infrastructure

- [x] GitHub repo (private): `meltforce/FreeReps`
- [x] CI: test.yml (Go build/test/lint + frontend typecheck/build)
- [x] CD: deploy.yml (Docker Hub edge + Tailscale SSH deploy)
- [x] Release: release.yml (CalVer tags, Docker Hub latest + versioned)
- [ ] Docker Hub repo: `meltforce/freereps`
- [ ] Tailscale OAuth secrets for CI deploy
- [ ] Production server setup

## Notes

- Go binary path on dev: `/usr/local/go/bin/go` (not in default PATH)
- `web/dist` must exist for `go build` (frontend must be built first)
- Active energy .hae files contain dual units (kJ + kcal) — importer filters to kcal only
- Sleep stages in .hae files use field names (awake/core/deep/rem) not a "value" string
- HR field names in .hae files are lowercase (min/avg/max) unlike REST API (Min/Avg/Max)
- Workout HR correlation runs after all imports, only for workouts with no existing HR data
- Version injected via `-ldflags "-X main.Version=..."` at build time
