# FreeReps

**FREE Records, Evaluation & Processing Server**

A self-hosted server that receives Apple Health data, stores it persistently, visualizes it through a web dashboard with freely configurable correlations, and exposes it as an MCP server for LLMs.

## Why FreeReps?

Apple Health collects extensive data but offers no way to relate metrics to each other, no API for external analysis, and no export into a queryable system you own.

Apps like Athlytic compute scores but are closed-source, subscription-based, and opaque. FreeReps takes the opposite approach: **raw data + flexible visualization + LLM for interpretation**.

## Architecture

```
┌──────────────┐     HTTP POST      ┌─────────────────────────────────────────┐
│ Health Auto   │ ────────────────→  │              FreeReps                   │
│ Export (iOS)  │    JSON/Batch      │                                         │
└──────────────┘                     │  ┌──────────┐  ┌─────────────────┐     │
                                     │  │ Ingest   │→ │  Storage (DB)   │     │
┌──────────────┐   freereps-upload   │  │ API      │  │  Time Series    │     │
│ .hae Files   │ ────────────────→   │  └──────────┘  └────────┬────────┘     │
│ (iCloud)     │    HTTPS/Tailscale  │                         │              │
└──────────────┘                     │              ┌──────────┴──────────┐   │
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

## Tech Stack

| Component | Technology |
|-----------|------------|
| Backend | Go (single binary with embedded frontend) |
| Frontend | React + Vite + Tailwind |
| Charts | uPlot (time-series) + Recharts (bar/scatter) |
| Database | PostgreSQL + TimescaleDB |
| Auth | Tailscale (tsnet) — zero-config TLS + identity |
| Config | YAML |
| Deployment | Docker Compose |

## Quick Start

### Server (Docker Compose)

```bash
git clone https://github.com/meltforce/FreeReps.git
cd FreeReps
cp config.example.yaml config.yaml
# Edit config.yaml — set database password, enable Tailscale
docker compose up -d
```

### Upload Tool (macOS)

`freereps-upload` is a client-side CLI tool that reads `.hae` files from your iCloud Drive (exported by [Health Auto Export](https://healthyapps.dev)), converts them to REST API format, and uploads them to your FreeReps server over Tailscale.

**Install:**

```bash
curl -sSL https://raw.githubusercontent.com/meltforce/FreeReps/main/scripts/install-upload.sh | bash
```

**Usage:**

```bash
# First run — upload all historical data
freereps-upload \
  -server https://freereps.your-tailnet.ts.net \
  -path ~/Library/Mobile\ Documents/com~apple~CloudDocs/Health\ Auto\ Export/AutoSync

# Subsequent runs — only new/changed files are uploaded (resumable)
freereps-upload \
  -server https://freereps.your-tailnet.ts.net \
  -path ~/Library/Mobile\ Documents/com~apple~CloudDocs/Health\ Auto\ Export/AutoSync
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-server` | (required) | FreeReps server URL |
| `-path` | (required) | Path to AutoSync directory (or parent) |
| `-dry-run` | false | Parse and convert without sending |
| `-batch-size` | 2000 | Data points per metric payload |
| `-version` | | Print version and exit |

**Requirements:** `lzfse` must be installed (`brew install lzfse`).

**Update / Uninstall:**

```bash
# Update to latest version
curl -sSL https://raw.githubusercontent.com/meltforce/FreeReps/main/scripts/install-upload.sh | bash -s -- --update

# Uninstall
curl -sSL https://raw.githubusercontent.com/meltforce/FreeReps/main/scripts/install-upload.sh | bash -s -- --uninstall
```

**State tracking:** Upload progress is tracked in `~/.freereps-upload/state.db` (SQLite). Files are identified by path + size + SHA-256 hash, so changed files are re-uploaded and the tool is fully resumable.

## Data Sources

### Health Auto Export (iOS)

The iOS app [Health Auto Export](https://healthyapps.dev) serves as the bridge between Apple Health and FreeReps.

- **REST API Automation**: The app pushes data via HTTP POST in JSON format
- **File Export**: Monthly `.hae` file exports via iCloud Drive, uploaded with `freereps-upload`

### Alpha Progression (iOS)

[Alpha Progression](https://alphaprogression.com) CSV exports provide detailed strength training data (exercises, sets, reps, weight, RIR).

Upload via the dashboard or POST to `/api/v1/ingest/alpha`.

## Dashboard Features

- **Daily Overview** — Key metrics at a glance (sleep, HRV, RHR, activity)
- **Correlation Explorer** — Plot any metric against any other (scatter + overlay, Pearson r)
- **Sleep View** — Hypnogram, stages, HR/HRV/SpO2 during sleep
- **Workout View** — HR zones, route map, Alpha Progression sets
- **Metrics Deep Dive** — Time-series with moving average, normal range band
- **Saved Views** — Store correlation configurations for quick recall

## MCP Server

FreeReps exposes health data to Claude (and other LLMs) via the Model Context Protocol.

**Tools:** `get_health_metrics`, `get_workouts`, `get_sleep_data`, `get_metric_stats`, `get_correlation`, `compare_periods`, `list_available_metrics`, `get_workout_sets`

**Resources:** `daily_summary`, `recent_workouts`, `metric_catalog`

### stdio (Claude Code)

```bash
freereps --mcp -config config.yaml
```

Add to your Claude Code MCP config:

```json
{
  "mcpServers": {
    "freereps": {
      "command": "/path/to/freereps",
      "args": ["--mcp", "-config", "/path/to/config.yaml"]
    }
  }
}
```

### SSE (Network)

The MCP SSE endpoint is automatically available at `/mcp/sse` when the server is running with Tailscale enabled.

## Supported Metrics

| Category | Metrics |
|----------|---------|
| Cardiovascular | heart_rate, resting_heart_rate, heart_rate_variability, blood_oxygen_saturation, respiratory_rate, vo2_max |
| Sleep | sleep_analysis, apple_sleeping_wrist_temperature |
| Body | weight_body_mass, body_fat_percentage |
| Activity | active_energy, basal_energy_burned, apple_exercise_time |
| Workouts | All types (with HR data + routes) |

## Design Principles

- **Privacy first** — All data stays local. No cloud uploads, no telemetry.
- **Self-hosted** — Runs on your own server/homelab.
- **Data over scores** — Raw data + visualization + LLM instead of proprietary algorithms.
- **Flexible over opinionated** — Correlation explorer instead of hard-wired dashboards.
- **Single binary** — Go binary with embedded web UI.

## API Reference

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/ingest/` | POST | Ingest Health Auto Export JSON |
| `/api/v1/ingest/alpha` | POST | Ingest Alpha Progression CSV |
| `/api/v1/metrics/latest` | GET | Latest value per metric |
| `/api/v1/metrics` | GET | Time-range metric query |
| `/api/v1/metrics/stats` | GET | Metric statistics (avg, min, max, stddev) |
| `/api/v1/timeseries` | GET | Time-bucketed metric data |
| `/api/v1/correlation` | GET | Pearson r between two metrics |
| `/api/v1/sleep` | GET | Sleep sessions + stages |
| `/api/v1/workouts` | GET | Workout list with filters |
| `/api/v1/workouts/{id}` | GET | Workout detail |
| `/api/v1/workouts/{id}/sets` | GET | Alpha Progression sets |
| `/api/v1/allowlist` | GET | Metric allowlist |
| `/api/v1/me` | GET | Current user identity |

## License

[MIT](LICENSE)
