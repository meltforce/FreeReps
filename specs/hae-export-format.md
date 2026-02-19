# Health Auto Export — File Export Format (.hae)

**Phase 1.5 reference** — not used in Phase 1 (REST API only).

## Overview

The iOS app can auto-sync health data to iCloud Drive as `.hae` files.
These are LZFSE-compressed JSON files (Apple's compression algorithm).

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
│   ├── walking_20251220_<UUID>.hae
│   ├── traditional_strength_training_20260108_<UUID>.hae
│   └── <type>_<YYYYMMDD>_<UUID>.hae
└── Routes/
    └── <UUID>.hae
```

## Key Differences from REST API

| Aspect | REST API | File Export |
|--------|----------|-------------|
| Timestamps | String `"yyyy-MM-dd HH:mm:ss Z"` | Apple epoch (seconds since 2001-01-01) |
| Workout HR | Inline `heartRateData` array | Separate heart_rate metric files |
| Workout Routes | Inline `route` array | Separate `Routes/<uuid>.hae` files |
| Values | Objects with units `{"qty": N, "units": "..."}` | Flat numbers |
| Compression | None (plain JSON) | LZFSE |

## Timestamp Conversion

Apple Core Data epoch: seconds since 2001-01-01 00:00:00 UTC.
To convert to Unix timestamp: add `978307200`.

```go
unixTimestamp := appleTimestamp + 978307200
```

## Decompression

LZFSE is Apple's proprietary compression. Options:
- `lzfse` CLI tool (Homebrew: `brew install lzfse`)
- Go library (TBD — needs research for Phase 1.5)

## Import Strategy (Phase 1.5)

1. Walk the AutoSync directory structure
2. Decompress each .hae file
3. Parse the JSON (different schema than REST)
4. Convert Apple timestamps to UTC
5. For workouts: correlate HR data from HealthMetrics/heart_rate/ by time overlap
6. For routes: match by workout UUID from Routes/ directory
7. Insert into same DB tables as REST API data
