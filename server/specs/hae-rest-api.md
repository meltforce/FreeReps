# Health Auto Export — REST API Format Spec

Reference: https://help.healthyapps.dev/en/health-auto-export/automations/rest-api/

## Overview

The iOS app "Health Auto Export" sends HTTP POST requests to a configured URL.
FreeReps receives these at `POST /api/v1/ingest`.

**Export Version**: 2 (current)
**Format**: JSON
**Content-Type**: `application/json`

## Request Headers

Automatic headers included in every request:

| Header | Description | Example |
|--------|-------------|---------|
| `Content-Type` | Always `application/json` | `application/json` |
| `automation-name` | User-defined name | `"FreeReps Health Metrics"` |
| `automation-id` | Unique automation identifier | `"abc123"` |
| `automation-aggregation` | Time grouping setting | `"minutes"` |
| `automation-period` | Date range setting | `"sinceLastSync"` |
| `session-id` | Unique per request | `"sess-456"` |

Custom headers configured by user (for auth):
- `X-API-Key: <key>` — FreeReps API key

## JSON Payload Structure

```json
{
  "data": {
    "metrics": [ ... ],
    "workouts": [ ... ]
  }
}
```

Root object has a single `data` key containing arrays for each data type.
Only the arrays relevant to the automation's data type selection are present.

Full list of possible arrays: `metrics`, `workouts`, `symptoms`, `ecg`,
`heartRateNotifications`, `stateOfMind`, `cycleTracking`, `medications`.

**V1 scope**: FreeReps only processes `metrics` and `workouts`.

## Date Format

All dates use: `yyyy-MM-dd HH:mm:ss Z`

Go layout: `"2006-01-02 15:04:05 -0700"`

Example: `"2024-02-06 14:30:00 -0800"`

Exception: aggregated sleep `date` field uses date-only: `"2024-02-06"`

## Health Metrics

Each metric is an object with `name`, `units`, and a `data` array.

### Standard Metric (qty)

Most metrics use a simple `qty` field:

```json
{
  "name": "resting_heart_rate",
  "units": "bpm",
  "data": [
    {
      "date": "2024-02-06 14:30:00 -0800",
      "qty": 58
    }
  ]
}
```

Metrics using this shape: `resting_heart_rate`, `heart_rate_variability`,
`blood_oxygen_saturation`, `respiratory_rate`, `vo2_max`, `weight_body_mass`,
`body_fat_percentage`, `active_energy`, `basal_energy_burned`,
`apple_exercise_time`, `apple_sleeping_wrist_temperature`.

### Heart Rate (Min/Avg/Max)

Heart rate uses capitalized `Min`, `Avg`, `Max` fields:

```json
{
  "name": "heart_rate",
  "units": "bpm",
  "data": [
    {
      "date": "2024-02-06 14:30:00 -0800",
      "Min": 65,
      "Avg": 72,
      "Max": 85
    }
  ]
}
```

### Blood Pressure (systolic/diastolic)

```json
{
  "name": "blood_pressure",
  "units": "mmHg",
  "data": [
    {
      "date": "2024-02-06 14:30:00 -0800",
      "systolic": 120,
      "diastolic": 80
    }
  ]
}
```

### Sleep Analysis — Aggregated (Summarize Data: ON)

```json
{
  "name": "sleep_analysis",
  "units": "hr",
  "data": [
    {
      "date": "2024-02-06",
      "totalSleep": 7.5,
      "asleep": 7.0,
      "core": 3.5,
      "deep": 1.5,
      "rem": 2.0,
      "sleepStart": "2024-02-05 23:00:00 -0800",
      "sleepEnd": "2024-02-06 06:30:00 -0800",
      "inBed": 8.0,
      "inBedStart": "2024-02-05 22:45:00 -0800",
      "inBedEnd": "2024-02-06 06:45:00 -0800"
    }
  ]
}
```

Detection: presence of `totalSleep` field.

### Sleep Analysis — Unaggregated (Summarize Data: OFF)

Individual sleep stage segments (for hypnogram):

```json
{
  "name": "sleep_analysis",
  "units": "hr",
  "data": [
    {
      "startDate": "2024-02-05 23:00:00 -0800",
      "endDate": "2024-02-05 23:30:00 -0800",
      "qty": 0.5,
      "value": "Core",
      "deep": 0.0,
      "rem": 0.0,
      "sleepStart": "2024-02-05 23:00:00 -0800",
      "sleepEnd": "2024-02-06 06:30:00 -0800",
      "inBed": 8.0,
      "inBedStart": "2024-02-05 22:45:00 -0800",
      "inBedEnd": "2024-02-06 06:45:00 -0800"
    }
  ]
}
```

Detection: presence of `startDate` field.

Sleep stage values: `"Awake"`, `"Asleep"`, `"In Bed"`, `"Core"`, `"REM"`, `"Deep"`, `"Unspecified"`

## Workouts (Version 2)

Each workout is a rich object:

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | UUID |
| `name` | string | Workout type (e.g. `"Running"`, `"Traditional Strength Training"`) |
| `start` | date string | Start time |
| `end` | date string | End time |
| `duration` | number | Duration in seconds |

### Optional Scalar Fields

| Field | Type | Description |
|-------|------|-------------|
| `location` | string | `"Indoor"`, `"Outdoor"`, `"Pool"`, `"Open Water"` |
| `isIndoor` | bool | Indoor flag |

### Optional Quantity Fields (object with qty + units)

| Field | Units |
|-------|-------|
| `activeEnergyBurned` | `"kcal"` |
| `totalEnergy` | `"kcal"` |
| `intensity` | `"MET"` |
| `distance` | `"mi"` or `"km"` |
| `speed` / `avgSpeed` / `maxSpeed` | `"mph"` or `"kmph"` |
| `elevationUp` / `elevationDown` | `"ft"` or `"m"` |
| `temperature` | `"degF"` or `"degC"` |
| `humidity` | `"%"` |
| `stepCadence` | `"spm"` |
| `flightsClimbed` | `"count"` |
| `swimCadence` | `"spm"` |
| `totalSwimmingStrokeCount` | `"count"` |
| `lapLength` | `"mi"` |

Quantity object shape: `{"qty": <number>, "units": "<string>"}`

### Heart Rate Summary

```json
{
  "heartRate": {
    "min": {"qty": 120, "units": "bpm"},
    "avg": {"qty": 150, "units": "bpm"},
    "max": {"qty": 175, "units": "bpm"}
  },
  "maxHeartRate": {"qty": 175, "units": "bpm"},
  "avgHeartRate": {"qty": 150, "units": "bpm"}
}
```

### Heart Rate Time Series

```json
{
  "heartRateData": [
    {
      "date": "2024-02-06 07:00:00 -0800",
      "Min": 120,
      "Avg": 150,
      "Max": 175,
      "units": "bpm",
      "source": "Apple Watch"
    }
  ],
  "heartRateRecovery": [
    {
      "date": "2024-02-06 07:30:00 -0800",
      "Min": 140,
      "Avg": 145,
      "Max": 150,
      "units": "bpm",
      "source": "Apple Watch"
    }
  ]
}
```

### Route Data

```json
{
  "route": [
    {
      "latitude": 37.7749,
      "longitude": -122.4194,
      "altitude": 50.5,
      "course": 45.0,
      "courseAccuracy": 5.0,
      "horizontalAccuracy": 10.0,
      "verticalAccuracy": 15.0,
      "timestamp": "2024-02-06 07:00:00 -0800",
      "speed": 7.0,
      "speedAccuracy": 0.5
    }
  ]
}
```

Route point fields:
- `latitude`, `longitude` — GPS coordinates (float64)
- `altitude` — meters (float64)
- `course` — degrees 0-360 (float64)
- `courseAccuracy`, `horizontalAccuracy`, `verticalAccuracy` — meters (float64)
- `timestamp` — date string
- `speed` — meters/second (float64)
- `speedAccuracy` — m/s (float64)

### Workout Metric Time Series

When "Include Workout Metrics" is enabled, these arrays may be present:

```json
{
  "activeEnergy": [{"date": "...", "qty": 50, "units": "kcal", "source": "Apple Watch"}],
  "basalEnergy": [{"date": "...", "qty": 20, "units": "kcal", "source": "Apple Watch"}],
  "stepCount": [{"date": "...", "qty": 5000, "units": "count", "source": "Apple Watch"}],
  "walkingAndRunningDistance": [{"date": "...", "qty": 0.25, "units": "mi", "source": "Apple Watch"}],
  "cyclingCadence": [{"date": "...", "qty": 90, "units": "rpm", "source": "Apple Watch"}],
  "cyclingDistance": [{"date": "...", "qty": 0.5, "units": "mi", "source": "Apple Watch"}],
  "cyclingPower": [{"date": "...", "qty": 200, "units": "W", "source": "Power Meter"}],
  "cyclingSpeed": [{"date": "...", "qty": 18, "units": "mph", "source": "Apple Watch"}],
  "swimDistance": [{"date": "...", "qty": 25, "units": "yd", "source": "Apple Watch"}],
  "swimStroke": [{"date": "...", "qty": 20, "units": "count", "source": "Apple Watch"}]
}
```

Each entry: `{date, qty, units, source?}`

## Batch Requests

When enabled, the app splits data across multiple HTTP requests.
Each request has the same headers. The `session-id` groups them.
FreeReps must handle each request independently (idempotent).

## Date Range Options

| Option | Behavior |
|--------|----------|
| Default | Full previous day + current date |
| Since Last Sync | Data since last successful sync |
| Today | Current date only |
| Yesterday | Previous date only |
| Previous 7 Days | Rolling 7-day window |

## Summarize Data

- **ON**: Aggregated data (daily summaries, min/avg/max for HR)
- **OFF**: Individual data points (each HR reading, each sleep stage)

FreeReps should handle both modes.
