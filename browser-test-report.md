# Browser Test Report

## 2026-02-21 — Workouts Tab

Tested on: `https://freereps-test.leo-royal.ts.net/`

### Test 1: Alpha Progression — German workout names (FAIL)

**Steps:**
1. Go to Workouts tab
2. Click a "Traditionelles Krafttraining" workout (Do., 19. Feb. 04:45)
3. Check workout detail page for exercise set data

**Expected:** Exercise sets (name, weight, reps, RIR) displayed on workout detail page.

**Actual:** Workout detail shows Duration, Active Cal, Avg/Max HR, Heart Rate Timeline, and Time in HR Zones only. No exercise sets anywhere on the page. Alpha Progression set data is silently missing.

**Verdict:** FAIL — strength training set data not rendered in workout detail view.

---

### Test 2: Filter state persistence (PASS)

**Steps:**
1. On Workouts tab, change time range to 30d
2. Select "Traditionelles Krafttraining" type filter
3. Verify URL shows `?range=30d&offset=0&type=Traditionelles+Krafttraining`
4. Click into a workout detail
5. Hit browser back button

**Expected:** Filters still active after back navigation.

**Actual:** URL correctly restored with all query params. 30d range highlighted, type filter highlighted, list shows only matching workouts.

**Verdict:** PASS

---

### Test 3: Workouts list API payload size (PASS, minor note)

**Steps:**
1. Load Workouts list (30d range, 60 workouts)
2. Inspect `/api/v1/workouts?start=...&end=...` response

**Expected:** No `raw_json` field in response objects.

**Actual:**
- Payload size: 30,510 bytes for 60 workouts — reasonable
- Field is named `RawJSON` (Go PascalCase), not `raw_json`
- Value is `null` for all 60 workouts — no data bloat
- Field key still serialized as `"RawJSON":null` on every object (could use `omitempty` to omit)

**Verdict:** PASS — no actual data bloat. Minor: `RawJSON` field present as null; `omitempty` would clean it up.
