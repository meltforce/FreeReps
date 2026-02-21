# Test Report — Workouts Tab Fixes

**Date**: 2026-02-21
**Server**: `https://freereps-test.leo-royal.ts.net/`
**Version**: v0.2

## Test 1: Alpha Progression exercise sets display

**Result: PASS**

- Opened Traditionelles Krafttraining workout from Feb 19 (04:45)
- "Exercises" section renders below the HR Zones chart
- 6 exercises displayed: Hack Squats, Sumo Squats, Hyperextensions on Roman Chair, Reverse Lunges, Standing Calf Raises, Hanging Leg Raises
- Each exercise shows: name, equipment type, Set/Weight/Reps/RIR columns
- Warmup sets marked with "W" in the Set column
- Bodyweight exercises show "BW" in the Weight column
- Data matches the `/api/v1/workouts/{id}/sets` endpoint

## Test 2: RawJSON suppressed from API responses

**Result: PASS**

- `/api/v1/workouts` (list, 8,449 bytes): no `RawJSON`, `rawJSON`, or `raw_json` field found
- `/api/v1/workouts/{id}` (detail, 13,967 bytes): no `RawJSON`, `rawJSON`, or `raw_json` field found

## Test 3: Filter state persistence (regression check)

**Result: FAIL → PASS (after fix)**

Initial run (pre-fix): filter reset to "All" when switching tabs. NavLink `to="/workouts"` navigated without query params.

After sessionStorage fix, re-tested all three scenarios:

### 3a: Type filter persists across tab switch — PASS
- Selected "Traditionelles Krafttraining" filter (URL: `?type=Traditionelles+Krafttraining`)
- Switched to Dashboard, switched back to Workouts
- Filter still active, URL restored with `?type=Traditionelles+Krafttraining`

### 3b: Time range persists across tab switch — PASS
- Changed time range to 30d while "Traditionelles Krafttraining" was active
- URL: `?type=Traditionelles+Krafttraining&range=30d&offset=0`
- Switched to Dashboard, switched back to Workouts
- Both type filter and 30d range persisted

### 3c: Cleared filter persists across tab switch — PASS
- Cleared type filter back to "All" (URL: `?range=30d&offset=0`, no type param)
- Switched to Dashboard, switched back to Workouts
- "All" remained selected, 30d range persisted, no stale type filter reappeared
