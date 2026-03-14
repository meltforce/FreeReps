# Health Auto Export — File Export Format (.hae)

**Phase 1.5 reference** — not used in Phase 1 (REST API only).

## Overview

The iOS app can auto-sync health data to iCloud Drive as `.hae` files.
These are LZFSE-compressed JSON files (Apple's compression algorithm, `bvx2` magic bytes).

## Directory Structure

```
AutoSync/
├── HealthMetrics/
│   ├── heart_rate/
│   │   ├── 20251222.hae
│   │   ├── 20251223.hae
│   │   └── ...
│   ├── sleep_analysis/
│   │   └── ...
│   └── <metric_name>/
│       └── <YYYYMMDD>.hae
├── Workouts/
│   ├── cycling_20251219_585BDA5C-5A64-4D5A-A432-6BCA6C7BCDBE.hae
│   ├── traditional_strength_training_20260108_<UUID>.hae
│   └── <type>_<YYYYMMDD>_<UUID>.hae
└── Routes/
    └── <UUID>.hae
```

## Key Differences from REST API

| Aspect | REST API | File Export |
|--------|----------|-------------|
| Timestamps | String `"yyyy-MM-dd HH:mm:ss Z"` | Apple epoch (float64, seconds since 2001-01-01) |
| Workout HR | Inline `heartRateData` array | Separate `heart_rate` metric files (correlate by time overlap) |
| Workout Routes | Inline `route` array | Separate `Routes/<uuid>.hae` files (match by workout UUID) |
| Values | Objects with units `{"qty": N, "units": "..."}` | Flat numbers (no unit wrappers) |
| Compression | None (plain JSON) | LZFSE (`bvx2` magic bytes) |
| HR field case | `Min`/`Avg`/`Max` (capitalized) | `min`/`avg`/`max` (lowercase) |

## Timestamp Conversion

Apple Core Data epoch: seconds since 2001-01-01 00:00:00 UTC.
To convert to Unix timestamp: add `978307200`.

```go
const appleEpochOffset = 978307200
unixTimestamp := appleTimestamp + appleEpochOffset
```

Timestamps are float64 (fractional seconds).

## JSON Schemas

### Health Metric File (`HealthMetrics/{metric_name}/{YYYYMMDD}.hae`)

```json
{
  "metric": "Heart Rate",
  "date": 788223600,
  "data": [
    {
      "metric": "Heart Rate",
      "start": 788223754,
      "end": 788223755,
      "unit": "count/min",
      "min": 59,
      "avg": 59,
      "max": 59,
      "sources": [
        {"name": "Linus Watch Ultra 2", "identifier": "com.apple.health.xxx"}
      ]
    }
  ]
}
```

**Standard metrics** (active_energy, resting_heart_rate, etc.) use `qty`:
```json
{
  "metric": "Resting Heart Rate",
  "date": 788050800,
  "data": [
    {
      "metric": "Resting Heart Rate",
      "start": 788050814.8258898,
      "end": 788137186.197352,
      "unit": "count/min",
      "qty": 70,
      "sources": [{"name": "...", "identifier": "..."}]
    }
  ]
}
```

**Heart rate** uses `min`/`avg`/`max` (lowercase, unlike REST API which uses capitalized):
```json
{
  "min": 59, "avg": 59, "max": 59
}
```

**Active energy** has dual-unit entries (kJ + kcal for same timestamp). Filter to `kcal` only:
```json
{"unit": "kJ", "qty": 0.012, "start": 788742341, ...},
{"unit": "kcal", "qty": 0.002, "start": 788742341, ...}
```

### Sleep Analysis File (`HealthMetrics/sleep_analysis/{YYYYMMDD}.hae`)

Each entry is a sleep stage segment. Stage type is determined by which field is present:

```json
{
  "metric": "Sleep Analysis",
  "data": [
    {
      "start": 788135607.886,
      "end": 788137608.019,
      "unit": "hr",
      "totalSleep": 0.555,
      "core": 0.555,
      "metric": "Sleep Analysis",
      "sources": [{"name": "...", "identifier": "..."}],
      "meta": {}
    },
    {
      "start": 788137608.019,
      "end": 788137846.841,
      "unit": "hr",
      "totalSleep": 0.066,
      "deep": 0.066,
      ...
    },
    {
      "awake": 0.033,
      ...
    }
  ]
}
```

Stage detection: whichever of `awake`, `core`, `deep`, `rem` is present and non-zero.
If none present, skip the entry.

### Workout File (`Workouts/{type}_{YYYYMMDD}_{UUID}.hae`)

Flat numbers, no unit wrappers:

```json
{
  "id": "585BDA5C-5A64-4D5A-A432-6BCA6C7BCDBE",
  "name": "Cycling",
  "start": 787833106.40769,
  "end": 787835103.202923,
  "duration": 1996.795,
  "activeEnergy": 235.264,
  "totalDistance": 4.768,
  "elevationUp": 25.93,
  "temperature": 7.652,
  "humidity": 81,
  "METs": 6.482,
  "location": "indoor"
}
```

Optional fields: `totalDistance`, `elevationUp`, `temperature`, `humidity`, `METs`, `location`.

### Route File (`Routes/{UUID}.hae`)

UUID matches the workout `id` field:

```json
{
  "id": "0EEA1E9E-C117-4BF7-A170-5C0B942CB69A",
  "name": "Hiking",
  "locations": [
    {
      "latitude": 52.634,
      "longitude": 13.276,
      "elevation": 64.257,
      "speed": 0.036,
      "course": 285.589,
      "time": 788182029.022,
      "hAcc": 6.366,
      "vAcc": 2.055
    }
  ]
}
```

## Decompression

LZFSE is Apple's proprietary compression. Use `lzfse` CLI tool:
```bash
lzfse -decode -i input.hae
```

The tool reads from stdin or `-i` and writes to stdout or `-o`.

## Import Strategy (Phase 1.5)

1. Walk the `AutoSync/` directory structure
2. Decompress each `.hae` file with `lzfse -decode`
3. Parse the JSON (different schema than REST — see above)
4. Convert Apple timestamps to UTC
5. Check metric against allowlist, skip if not allowed
6. For health metrics: batch insert into `health_metrics` table
7. For sleep: insert sleep stage segments into `sleep_stages` table
8. For workouts: insert into `workouts` table, match routes by UUID
9. For HR correlation: after all data imported, query `heart_rate` data overlapping each workout and insert into `workout_heart_rate`
10. Dedup via existing `ON CONFLICT DO NOTHING` on all tables
