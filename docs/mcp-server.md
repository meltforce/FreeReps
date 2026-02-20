# FreeReps MCP Server

FreeReps exposes your health data via the [Model Context Protocol](https://modelcontextprotocol.io/) so Claude (or any MCP client) can query it directly.

Two transports are available:

- **stdio** — for local Claude Code integration (pipe JSON-RPC over stdin/stdout)
- **SSE** — served on the existing HTTP server at `/mcp`, inherits Tailscale authentication

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

### Claude Desktop or remote clients (SSE)

Connect your MCP client to:

```
https://freereps.<your-tailnet>/mcp/sse
```

SSE runs through the same Tailscale-authenticated HTTP server, so each user sees only their own data.

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

Strength training set data from Alpha Progression.

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

## Available Resources

| URI | Description |
|-----|-------------|
| `freereps://daily_summary` | Today's key metrics, latest sleep, daily activity totals |
| `freereps://recent_workouts` | Workouts from the last 14 days |
| `freereps://metric_catalog` | All available metrics with categories |

## Metric Names Reference

| Category | Metrics |
|----------|---------|
| Cardiovascular | `heart_rate`, `resting_heart_rate`, `heart_rate_variability`, `blood_oxygen_saturation`, `respiratory_rate`, `vo2_max` |
| Sleep | `sleep_analysis`, `apple_sleeping_wrist_temperature` |
| Body | `weight_body_mass`, `body_fat_percentage` |
| Activity | `active_energy`, `basal_energy_burned`, `apple_exercise_time` |

## Example Prompts

Once connected, you can ask Claude things like:

- "What's my resting heart rate trend over the last month?"
- "How does my sleep duration correlate with HRV?"
- "Compare my workout volume this week vs last week"
- "Show me my deep sleep trends for the past 90 days"
- "What exercises did I do in my last strength training session?"
