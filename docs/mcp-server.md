# FreeReps MCP Server

FreeReps exposes your health data via the [Model Context Protocol](https://modelcontextprotocol.io/) so Claude (or any MCP client) can query it directly.

Two transports are available:

- **stdio** — for local Claude Code integration (pipe JSON-RPC over stdin/stdout)
- **Streamable HTTP** — served at `/mcp` by the same HTTP server as the
  dashboard, behind the same authentication (README § Access)

There is no SSE endpoint. `/mcp/sse` matches no route, so the request reaches
the dashboard handler and the response is HTML.

## Setup

### Claude Code (stdio)

Add to your Claude Code MCP config (`~/.claude/claude_code_config.json` or project-level `.mcp.json`):

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

stdio mode always runs as user_id=1 (the local/default user).

### Claude Desktop or remote clients (Streamable HTTP)

Connect your MCP client to:

```
https://freereps.<your-tailnet>/mcp
```

A client that speaks only stdio reaches it through
[mcp-proxy](https://github.com/sparfenyuk/mcp-proxy):

```json
{
  "mcpServers": {
    "freereps": {
      "command": "mcp-proxy",
      "args": ["--transport", "streamablehttp", "https://freereps.<your-tailnet>/mcp"]
    }
  }
}
```

The endpoint runs through the same authenticated HTTP server, so each user sees only their own data. With application login the client sends a personal access token with scope `mcp` as `Authorization: Bearer <token>`.

## Available Tools

### get_health_metrics

Retrieve time-bucketed health metrics (avg/min/max per bucket).

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `metric` | yes | — | Metric name (e.g. `heart_rate`, `resting_heart_rate`, `heart_rate_variability`) |
| `start` | no | 7 days ago | Start date (`YYYY-MM-DD` or ISO 8601) |
| `end` | no | now | End date |
| `bucket` | no | `1 day` | Aggregation bucket: `1 hour`, `1 day`, `1 week` |

### get_metric_stats

Aggregate statistics for a single metric over a time range.

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `metric` | yes | — | Metric name |
| `start` | no | 7 days ago | Start date |
| `end` | no | now | End date |

Returns: `avg`, `min`, `max`, `stddev`, `count`.

### get_correlation

Pearson correlation between two metrics.

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `x` | yes | — | X-axis metric |
| `y` | yes | — | Y-axis metric |
| `start` | no | 7 days ago | Start date |
| `end` | no | now | End date |
| `bucket` | no | `1 day` | Time bucket for alignment |

Returns: paired data points and `pearson_r` coefficient.

### get_sleep_data

Sleep sessions and individual stage segments.

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `start` | no | 7 days ago | Start date |
| `end` | no | now | End date |

Returns: `sessions` (nightly summaries with total/core/deep/REM hours) and `stages` (individual segments with start/end times).

### get_workouts

Workout summaries with optional type filter.

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `start` | no | 7 days ago | Start date |
| `end` | no | now | End date |
| `type` | no | all | Workout type (e.g. `Traditional Strength Training`, `Outdoor Walk`, `Yoga`) |

### get_workout_sets

Strength training set data from Hevy and from imported Alpha Progression exports.

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `start` | no | 7 days ago | Start date |
| `end` | no | now | End date |

Returns per-set detail: exercise name, weight, reps, RIR, equipment.

### compare_periods

Compare a metric's statistics between two time periods.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `metric` | yes | Metric name |
| `period_a_start` | yes | Period A start date |
| `period_a_end` | yes | Period A end date |
| `period_b_start` | yes | Period B start date |
| `period_b_end` | yes | Period B end date |

Returns stats (avg/min/max/stddev/count) for each period.

### list_available_metrics

Lists all tracked metrics with category and enabled status. No parameters.

### get_strength_summary

Workout and strength volume aggregated per week or month, across every logging
source: workout counts, duration and calories by type, plus set, rep and tonnage
totals per period. In the strength block, `sessions` counts distinct session
start times and is the denominator of `avg_sets_per_session`; `training_days`
counts calendar days on which anything was logged.

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `start` | no | 6 months ago | Start date |
| `end` | no | now | End date |

### get_strength_volume

Sets per muscle group per period, both as primary-target sets and as sets
weighted with assisting muscles at 0.5, plus training frequency per muscle. It
also reports how much of each figure rests on approximate exercise mapping, or
on exercises with no muscle data at all.

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `start` | no | 12 weeks ago | Start date |
| `end` | no | now | End date |

### get_strength_intensity

Effort distribution in reps-in-reserve bands, failure rate, per-exercise stats
and optional per-session progression. Sets logged as RPE and as RIR are both
covered.

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `start` | no | 90 days ago | Start date |
| `end` | no | now | End date |

### get_strength_1rm

Estimated one-rep max per exercise per session, Epley over repetitions plus
reps in reserve. Sets without an effort rating are excluded, and the estimate
loses accuracy above roughly ten effective repetitions.

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `start` | no | 6 months ago | Start date |
| `end` | no | now | End date |

### get_sleep_summary

Aggregated sleep statistics per period: duration, stage percentages,
efficiency, and bedtime and waketime consistency.

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `start` | no | 90 days ago | Start date |
| `end` | no | now | End date |

### Record queries

Six tools read the record tables that are not time series of a single number.
Each takes an optional `start` (ISO 8601 or `YYYY-MM-DD`, default 7 days ago);
`get_category_samples` also takes `end`.

| Tool | Returns |
|------|---------|
| `get_ecg_recordings` | id, classification, average heart rate, start date, source |
| `get_audiograms` | id, sensitivity points, start date, source |
| `get_activity_summaries` | date, active energy, exercise time, stand hours, and their goals |
| `get_medications` | id, name, dosage, log status, start date, source |
| `get_vision_prescriptions` | every prescription field, including per-eye detail |
| `get_state_of_mind` | id, kind, valence, labels, associations, start date |
| `get_category_samples` | id, type, value, value label, start and end date |

## Available Resources

| URI | Description |
|-----|-------------|
| `freereps://daily_summary` | Today's key metrics, latest sleep, daily activity totals |
| `freereps://recent_workouts` | Workouts from the last 14 days |
| `freereps://metric_catalog` | All available metrics with categories |

## Metric Names Reference

`list_available_metrics` returns the names the instance actually carries, with
category and enabled status. The README's
[Supported Metrics](../README.md#supported-metrics) table names the groups.

## Example Prompts

Once connected, you can ask Claude things like:

- "What's my resting heart rate trend over the last month?"
- "How does my sleep duration correlate with HRV?"
- "Compare my workout volume this week vs last week"
- "Show me my deep sleep trends for the past 90 days"
- "What exercises did I do in my last strength training session?"
