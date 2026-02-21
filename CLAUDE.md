# FreeReps

**FREE Records, Evaluation & Processing Server**

- **Repo**: `FreeReps`
- **Binary**: `freereps`

## Project Vision

FreeReps is a self-hosted server that receives Apple Health data, stores it persistently, visualizes it through a web dashboard with freely configurable relations/correlations, and exposes it as an MCP server for LLMs.

**Core idea**: Collect the data, store it cleanly, visualize it flexibly — and delegate intelligent analysis to Claude (via MCP). No built-in AI coach, no proprietary score algorithms. Instead: maximum transparency and flexibility in data exploration.

**Visual inspiration**: [Athlytic](https://www.athlyticapp.com/) — but as a self-hosted web app, without scores, with free correlation views and an MCP interface instead.

## Problem Statement

Apple Health collects extensive data but offers:
- No way to relate metrics to each other (e.g. sleep duration ↔ HRV ↔ strength gains)
- No API for external analysis or LLM integration
- No useful trend analysis over longer time periods
- No export into a queryable system you own

Apps like Athlytic compute scores but are closed-source, subscription-based, and their algorithms are opaque. FreeReps takes the opposite approach: raw data + flexible visualization + LLM for interpretation.

## Data Source: Health Auto Export

The iOS app **[Health Auto Export](https://healthyapps.dev)** serves as the bridge between Apple Health and FreeReps.
The iOS app **[Alpha Progression](https://alphaprogression.com/de/)** serves as source for detailed strength training workout data (example data available in input/).

### Integration

- **REST API Automation** (primary path): The app pushes data via HTTP POST to FreeReps.
  - Format: JSON, Export Version 2
  - Configurable metrics, time ranges, aggregation
  - Batch requests supported
  - Custom HTTP headers for auth
  - Automatic sync cadence configurable
  - Manual export for historical data
  - Date range options: Default, Since Last Sync, Today, Yesterday, Previous 7 Days
  - Summarize Data: ON (aggregated) or OFF (individual data points)
  - Time grouping: seconds, minutes, hours, days
- **TCP/MCP Server Connection** (optional): App's own MCP server on the iPhone (port 9000). Limited: app must be in foreground, unencrypted, LAN only.

### Supported Data Types

| Data Type | Example Metrics |
|---|---|
| **Health Metrics** | Steps, HR, HRV, RHR, sleep stages, weight, SpO2, respiratory rate, temperature, calories |
| **Workouts** | Type, duration, distance, calories, HR data during workout, optional route |
| **Symptoms** | Health symptoms |
| **ECG** | Electrocardiogram recordings |
| **HR Notifications** | High/low heart rate events |
| **State of Mind** | Mood data (iOS 18.0+) |
| **Cycle Tracking** | Cycle data |
| **Medications** | Medication logs (iOS 26.0+) |

### Reference Documentation

- [REST API Automation](https://help.healthyapps.dev/en/health-auto-export/automations/rest-api/)
- [Server Connection (TCP/MCP)](https://help.healthyapps.dev/en/health-auto-export/automations/server-connection/)
- [Export Formats](https://help.healthyapps.dev/en/health-auto-export/export-format/)
- [GitHub Server (Grafana reference)](https://github.com/HealthyApps/health-auto-export-server)

## Architecture

```
┌──────────────┐     HTTP POST      ┌─────────────────────────────────────────┐
│ Health Auto   │ ────────────────→  │              FreeReps                   │
│ Export (iOS)  │    JSON/Batch      │                                         │
└──────────────┘                     │  ┌──────────┐  ┌─────────────────┐     │
                                     │  │ Ingest   │→ │  Storage (DB)   │     │
                                     │  │ API      │  │  Time Series    │     │
                                     │  └──────────┘  └────────┬────────┘     │
                                     │                         │              │
                                     │              ┌──────────┴──────────┐   │
                                     │              ▼                     ▼   │
                                     │  ┌────────────────┐  ┌─────────────┐  │
                                     │  │ Web Dashboard  │  │ MCP Server  │  │
                                     │  │ Correlations   │  │ stdio / SSE │  │
                                     │  │ Trends, Charts │  └──────┬──────┘  │
                                     │  └────────────────┘         │         │
                                     └─────────────────────────────┼─────────┘
                                                                   ▼
                                                          Claude (via MCP)
                                                          = the actual
                                                            "AI coach"
```

## Core Components

### 1. Ingest API

- HTTP POST endpoint compatible with Health Auto Export REST API Automation
- Supports headers: `automation-id`, `automation-name`, `session-id`, `automation-aggregation`, `automation-period`
- API key authentication
- Idempotent processing (deduplication via timestamp + metric type + source)
- Batch request support
- Separate automations per data type (Health Metrics, Workouts, etc.)

### 2. Storage

- Persistent time-series database for all received data
- Schema maps the Health Auto Export JSON structure
- Optimized for:
  - Time-range queries (last day, week, month, custom)
  - Cross-metric queries (for correlation views)
  - Rolling averages and aggregates
- Raw data is kept unchanged (no lossy preprocessing)

### 3. Web Dashboard

The heart of the user experience. Not a rigid dashboard, but a **flexible exploration tool** for your own health data.

#### Dashboard Home
- **Daily overview**: Key metrics at a glance
  - Last night: sleep duration, sleep stages, HRV during sleep
  - Today: steps, active calories, resting heart rate
  - Last workout: type, duration, average HR
- **Quick time series**: Sparklines / mini-charts of the last 7 days for core metrics

#### Correlation Explorer (Core Feature)
- **Freely selectable X/Y axes**: Any available metric can be plotted against any other
  - e.g. sleep duration (X) vs. HRV next morning (Y)
  - e.g. weekly training volume (X) vs. resting HR trend (Y)
  - e.g. sleep quality (X) vs. workout performance next day (Y)
- **Time series overlay**: Stack multiple metrics on the same time axis
  - e.g. HRV + sleep duration + training load in the same chart
  - Dual Y-axes for metrics with different scales
- **Configurable time range**: 7d, 30d, 90d, 1y, custom
- **Selectable aggregation**: Raw, daily average, weekly average
- **Scatter plot + trend line**: For correlation display with optional regression line
- **Saveable views**: Store correlation configurations as named views for quick recall

#### Sleep View
- **Hypnogram**: Sleep stage progression (Awake, REM, Light, Deep) as step chart
- **HR / HRV / SpO2 / respiratory rate** during sleep as overlay charts
- **Sleep metrics**: Duration, efficiency (sleep time / time in bed), latency, interruptions
- **Historical comparison**: Last night vs. 30-day average
- **Consistency chart**: Sleep/wake times over weeks

#### Workout View
- **Workout list**: Chronological, filterable by type
- **Workout detail**:
  - HR timeline with zone coloring (Z1–Z5)
  - Time per HR zone (bar chart)
  - Calories, duration, distance (if available)
  - Route on map (if available)
- **Workout comparison**: Compare same workout types over time (e.g. all running workouts)

#### Metrics View
- **Single-metric deep dive**: Any metric as time series with:
  - Daily values + moving average (7d, 30d)
  - Min/max/avg per time range
  - Personal normal range (e.g. mean ± 1 standard deviation, auto-calculated)
  - Outlier highlighting
- **Metrics**: HRV, RHR, weight, SpO2, respiratory rate, temperature, steps, calories, etc.

#### Trends View
- **Multi-metric trends**: Long-term development of freely selectable metrics
- **Weekly / monthly summaries**: Aggregated display over longer time periods

#### UI Guidelines
- **First-class desktop experience**: make use of the big screen for better overview during analysis
- **Practical mobile accesst**: Primarily used on phone, must work on small screens
- **Dark mode default**: Fitness app aesthetic
- **Color coding**: Consistent color scheme for HR zones, sleep stages, outliers
- **Responsive charts**: Touch-friendly, zoomable, pannable on time axis
- **Fast**: No waiting for dashboard load — data pre-computed or cached

### 4. MCP Server

Model Context Protocol server — makes FreeReps data queryable for Claude (and other LLMs).

**Transport**: stdio (local Claude Code integration) and/or SSE (remote via network)

**MCP Tools**:

| Tool | Description |
|---|---|
| `get_health_metrics` | Retrieve health metrics for a time range (metric, period, aggregation selectable) |
| `get_workouts` | Query workouts (type filter, time range, with/without HR data) |
| `get_sleep_data` | Sleep data including stages for a time range |
| `get_metric_stats` | Statistics for a metric (avg, min, max, stddev, trend) |
| `get_correlation` | Compute correlation between two metrics (Pearson r, time range) |
| `compare_periods` | Compare two time periods (e.g. this week vs. last week) |
| `get_workout_sets` | Strength training set data from Alpha Progression (exercises, weight, reps, RIR) |
| `list_available_metrics` | Which metrics are available + data availability time range |

**MCP Resources**:
- `daily_summary` — Summary of the current day
- `recent_workouts` — Recent workouts
- `metric_catalog` — Available metrics with metadata

**Philosophy**: MCP tools deliver data and basic statistics. Interpretation, recommendations, and "coaching" are done by Claude based on this data. This keeps FreeReps lean and the analysis flexible.

## Design Principles

- **Privacy first**: All data stays local. No cloud uploads, no telemetry.
- **Self-hosted**: Runs on your own server/homelab (Docker-compatible).
- **Data over scores**: Raw data + visualization + LLM instead of proprietary algorithms.
- **Flexible over opinionated**: Correlation explorer instead of hard-wired dashboards.
- **Single binary** (if possible): `freereps` with embedded web UI.
- **File-based configuration**: YAML/TOML for server, DB, auth, personal parameters (age, HR zones).
- **Idempotent ingest**: Duplicate data → no duplicates stored.

## Architecture Decisions

| Decision | Choice | Rationale |
|---|---|---|
| **Backend** | Go 1.25+ | Single binary, fast development, good concurrency |
| **Frontend** | React 19 + Vite + Tailwind CSS 4 | Large ecosystem, TypeScript, rich chart libraries |
| **Charts** | uPlot (time-series) + Recharts (bar/scatter) | uPlot for performance on large datasets, Recharts for declarative composability |
| **Database** | PostgreSQL + TimescaleDB | Time-series optimized, hypertables, rolling aggregates |
| **MCP Transport** | stdio + SSE | stdio for local Claude Code, SSE for remote/Tailscale access |
| **Deployment** | Docker Compose | PostgreSQL + app in one stack, multi-stage build |
| **Auth** | Tailscale tsnet | Zero-config TLS + identity, no passwords |

## Non-Goals (v1)

- Computed composite scores (Recovery, Exertion, etc.) — Claude can do this on demand via MCP
- Native iOS/watchOS app
- Direct Apple HealthKit integration
- Multi-user support
- Workout planning or automated coaching
- Third-party app integration (Strava, etc.)
- Push notifications

## Rules

Read specs/ before implementing. Specs are the source of truth.
Search before writing. Don't assume something is missing — ripgrep the codebase first.
One thing at a time. Implement, test, lint, commit. Then move on.
Lint before committing. Run `go vet ./...` and `golangci-lint run ./...` (or `make lint`) before every commit. Fix all issues first.
No placeholders. Full implementations only. No TODO stubs.
Tests are mandatory. Every new function gets a test. Test doc comments must explain WHY the test exists.
Update fix_plan.md after completing a task or discovering a bug.
Update this file when you learn something about building/running the project.
Commit after each unit of work with a descriptive message.

## Development Roadmap

### Phase 1 — Data Foundation ✅
- Ingest API (Health Auto Export-compatible)
- Database schema + storage layer
- Minimal web UI: daily overview + single time-series charts
- CLI upload tool (`freereps-upload`) for .hae files and TCP streaming

### Phase 2 — Visualization ✅
- Correlation Explorer (freely selectable X/Y, scatter + overlay)
- Sleep view with hypnogram
- Workout view with HR zones + Alpha Progression sets
- Metrics deep dive with normal range

### Phase 3 — Tailscale + User Management ✅
- Tailscale tsnet integration (zero-config TLS + identity)
- User context scoping for all data
- Settings page (identity, stats, import logs, uploads)

### Phase 4 — MCP Integration ✅
- MCP server (stdio + SSE)
- 8 tools + 3 resources
- Full data query and analytics API

### Phase 5 — Polish (not started)
- Trend views
- Saveable dashboard configurations
- Responsive optimization

## References

- [Athlytic App](https://www.athlyticapp.com/) — Visual inspiration
- [Health Auto Export – Docs](https://help.healthyapps.dev/en/)
- [Health Auto Export – REST API](https://help.healthyapps.dev/en/health-auto-export/automations/rest-api/)
- [Health Auto Export – Server Connection](https://help.healthyapps.dev/en/health-auto-export/automations/server-connection/)
- [Health Auto Export – Export Formats](https://help.healthyapps.dev/en/health-auto-export/export-format/)
- [Health Auto Export – GitHub Server](https://github.com/HealthyApps/health-auto-export-server)
- [MCP Specification](https://modelcontextprotocol.io/)
