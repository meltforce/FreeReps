# FreeReps Fix Plan

## Phase Status

| Phase | Status |
|-------|--------|
| Phase 1 — Data Foundation | Complete |
| Phase 1.5 — CLI File Import (server-side) | Complete |
| Phase 1.6 — Client-Side Upload Tool | Complete |
| Phase 2 — Visualization | Complete |
| Phase 3 — Tailscale tsnet + User Management | Complete |
| Phase 4 — MCP Integration | Complete |
| Phase 5 — Polish | Not started |

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

## Phase 1.6 — Client-Side Upload Tool (Complete)

- [x] SQLite state DB for tracking uploaded files (`internal/upload/state.go`)
- [x] .hae → REST API conversion (`internal/upload/convert.go`)
- [x] HTTP client with retry (`internal/upload/client.go`)
- [x] Upload pipeline orchestration (`internal/upload/upload.go`)
- [x] CLI entry point (`cmd/freereps-upload/main.go`)
- [x] Tests for conversions + HR correlation (`internal/upload/convert_test.go`)
- [x] Pure Go SQLite (`modernc.org/sqlite`) — no CGO, cross-compiles for macOS
- [x] macOS cross-compilation verified (darwin/arm64 + darwin/amd64)
- [x] README.md + MIT LICENSE
- [x] Install/update/uninstall script (`scripts/install-upload.sh`)
- [x] Release workflow updated with binary build matrix

## Phase 2 — Visualization (Complete)

- [x] Navigation + routing (React Router)
- [x] Dashboard home (daily overview + time-series)
- [x] Sleep view (hypnogram + metric cards + history chart)
- [x] Workout view (list + detail with HR zones, route map, Alpha sets)
- [x] Metrics deep dive (stats, moving average, normal range band)
- [x] Correlation explorer (scatter, overlay, saved views, Pearson r)
- [x] Error boundaries + code splitting
- [x] Bug fix rounds R1-R6 (cumulative metrics, sleep backfill, Alpha parser, list UX)

## Phase 3 — Tailscale tsnet + User Management (Complete)

- [x] Users table migration (000002)
- [x] Config: replace AuthConfig with TailscaleConfig (enabled, hostname, state_dir)
- [x] Add tsnet dependency (tailscale.com/tsnet)
- [x] User storage layer (GetOrCreateUser)
- [x] TailscaleIdentity middleware (WhoIs, tagged device rejection, user auto-provisioning)
- [x] DevIdentity middleware (user_id=1 fallback for local dev)
- [x] Remove APIKeyAuth middleware
- [x] Replace all hardcoded user_id=1 in handlers with userIDFromContext
- [x] Thread userID through HAE ingest provider (~6 places)
- [x] Thread userID through Alpha ingest provider
- [x] Conditional tsnet vs plain HTTP startup in main.go
- [x] Docker compose: tsnet-state volume, TS_AUTHKEY env, remove port mapping
- [x] Update tests (config, middleware, shapes)
- [x] Go 1.25 upgrade (required by tsnet)

## Phase 4 — MCP Integration (Complete)

- [x] Add `mark3labs/mcp-go` dependency (v0.44.0)
- [x] MCP server setup (stdio + SSE transport)
- [x] User identity context (`/api/v1/me` endpoint + frontend display)
- [x] Tool: `get_health_metrics` — retrieve time-bucketed metrics
- [x] Tool: `get_workouts` — query workouts with type filter
- [x] Tool: `get_sleep_data` — sleep sessions + stages
- [x] Tool: `get_metric_stats` — avg/min/max/stddev/count
- [x] Tool: `get_correlation` — Pearson r between two metrics
- [x] Tool: `compare_periods` — compare two time ranges
- [x] Tool: `list_available_metrics` — metric catalog
- [x] Tool: `get_workout_sets` — Alpha Progression set data
- [x] Resource: `daily_summary`
- [x] Resource: `recent_workouts`
- [x] Resource: `metric_catalog`
- [x] SSE transport mounted at `/mcp` (gets Tailscale auth for free)
- [x] stdio transport via `--mcp` flag (defaults to user_id=1)
- [x] Tests for MCP context helpers, handleMe, middleware

## Phase 5 — Polish

- [ ] Trend views (multi-metric long-term analysis)
- [ ] Settings UI (metric enable/disable, personal parameters)
- [ ] Saveable dashboard layouts (beyond correlation saved views)
- [ ] Responsive refinement

## Infrastructure

- [x] GitHub repo (private): `meltforce/FreeReps`
- [x] CI: test.yml (Go build/test/lint + frontend typecheck/build)
- [x] CD: deploy.yml (Docker Hub edge + Tailscale SSH deploy)
- [x] Release: release.yml (CalVer tags, Docker Hub latest + versioned, macOS binaries)
- [x] Open source prep: README.md, MIT LICENSE, install script
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
- tsnet requires Go 1.25+ (tailscale.com v1.94.2)
- tsnet enabled by default; set `tailscale.enabled: false` for local dev
- TS_AUTHKEY env var used for initial Tailscale node registration (tsnet reads it automatically)
- `freereps-upload` uses `modernc.org/sqlite` (pure Go) — no CGO required for cross-compilation
- Upload tool state DB at `~/.freereps-upload/state.db` — tracks files by path + size + SHA-256
- Upload tool does NOT depend on `internal/storage` or `pgx` — only `internal/models` + stdlib
