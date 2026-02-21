# Test Plan — Workouts Tab Fixes

Test server: `https://freereps-test.leo-royal.ts.net/`

## Test 1: Alpha Progression exercise sets display

1. Navigate to **Workouts** tab
2. Open a **Traditionelles Krafttraining** workout (e.g. from Feb 19)
3. Scroll down to the exercise sets section
4. **Expected**: Exercise sets appear with exercise names, set/rep/weight data, warmup sets marked
5. **Pass criteria**: At least one exercise with working sets and weights is visible

## Test 2: RawJSON suppressed from API responses

1. Open browser DevTools (Network tab)
2. Navigate to **Workouts** tab — observe the `/api/v1/workouts` response
3. **Expected**: No `RawJSON` field in any workout object
4. Also check a workout detail response (`/api/v1/workouts/<id>`)
5. **Pass criteria**: `RawJSON` does not appear anywhere in the JSON responses

## Test 3: Filter state persistence (regression check)

1. Navigate to **Workouts** tab
2. Set a filter (e.g. workout type = "Traditionelles Krafttraining")
3. Switch to another tab (e.g. Overview), then switch back to Workouts
4. **Expected**: Filter is still applied, same workout list shown
5. **Pass criteria**: Filter persists across tab switches
