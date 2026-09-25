# Incidents

Postmortems for things that broke. One section per incident, newest first.

Add an entry after fixing something that was not obvious — the kind of failure
where the useful question six months later is "have I seen this before?". Skip
routine config changes, dependency bumps, and one-line typos.

Structure per entry: **symptoms** (what was visible), **root cause** (concrete:
component, version, why), **fix** (what changed), **lesson** (one line,
actionable).

The entries below were written on 2026-08-04 from the commit history. Their
symptoms are as recorded in the commit messages; where a commit did not say how
the fix was verified, this file does not claim it was.

---

## 2026-09-25 — Hourly sums stayed at the value of the first sync inside the hour

**Symptoms.** The 12:00Z `step_count` bucket of 2026-09-25 (source `''`, written
by the iOS app) held 230.94 steps. Apple Health showed 413 for that hour. A sync
at 12:44Z delivered the complete hour — 200 ingest calls, all `success` — and the
stored value did not change.

**Root cause.** The iOS app sends 11 cumulative types as hourly sums
(`queryCumulativeStatistics`, `syncStrategy: .aggregateCumulative(interval:
3600)`), and every sync includes the hour still in progress. The first sync at
12:22Z stored the sum up to that minute. `InsertHealthMetrics` wrote with
`ON CONFLICT DO NOTHING` on `(metric_name, source, time, user_id)`, so every
later delivery of the same hour was discarded without an error, including the
7-day re-send each sync performs. Every hour in which a sync ran is affected for
steps, active and basal energy, the three distances, exercise, move and stand
time, flights climbed and swimming strokes. How many past hours are affected
depends on when syncs ran and was not measured.

**Fix.** `InsertHealthMetrics` upserts: `ON CONFLICT … DO UPDATE` with a
`WHERE … IS DISTINCT FROM` guard, so a changed bucket replaces the stored one and
an identical one writes nothing. Rows repeating a key within one call are
reduced to the last, because `DO UPDATE` rejects a statement that touches one
row twice. `health_metrics_upsert_integration_test.go` reproduces the 230.94 /
413 case. Past hours are corrected by re-sending them from the app after the
deploy; the upsert does not repair them on its own.

**Lesson.** A key that identifies a bucket rather than a measurement needs an
upsert; insert-or-skip keeps whichever version of the bucket arrived first.

---

## 2026-09-21 — No workout of the current day arrived, from either source

**Symptoms.** On 2026-09-21 the Workouts screen showed nothing from that day,
although two workouts had been recorded. Measured against the deployed instance
(`edge-f7b66e2`) at 17:00 UTC: `workouts` held no row after
2026-09-20 16:56:38Z, `workout_heart_rate` and `workout_routes` none after
2026-09-20 17:13Z, and Apple Health `health_metrics` none after
2026-09-20 08:23:01Z. Oura sleep, readiness, stress and heart rate continued to
arrive, the newest at 2026-09-21 12:00Z, which is what made the gap look
specific to workouts.

**Root cause.** Two independent causes, one per channel, with identical
symptoms.

*Oura.* `fetchAll` in `internal/oura/client.go` passed the caller's range
straight through, and `syncDataType` ends its range at `time.Now()`. On
`/v2/usercollection/workout` and `/v2/usercollection/daily_activity` the API
compares `end_date` against a timestamp, so a record of the end day itself is
left out. Measured against the live API on 2026-09-21:
`start_date=2026-09-19&end_date=2026-09-21` returned the two workouts of the
19th and neither of the 21st, `end_date=2026-09-22` returned all four;
`daily_activity` returned 2026-09-20 only until `end_date` reached 2026-09-22.
`daily_sleep`, `daily_readiness`, `daily_stress`, `daily_spo2`,
`daily_resilience`, `daily_cardiovascular_age` and `sleep` returned the end day
in the same measurement, which is why sleep never showed the defect. The
two-day overlap in `syncDataType` hid the rest: the current day's workouts and
activity arrived on the following day's first sync, so only the current day was
ever missing, on every day since the Oura integration was added.

*Health Auto Export.* The export window in the phone's automation was set to
"previous 7 days", which ends on the previous day. The deliveries of
2026-09-21 at 05:19, 12:24 and 16:18 each carried the same 24 workouts of
2026-09-14 to 2026-09-20, 0 inserted; the delivery at 05:17 wrote the 4 of
2026-09-20. This is configuration on the phone rather than code, and it is
recorded here because the server-side symptom is indistinguishable from a
defect in the ingest path.

*The rule that should have reported it.* `checkAppleIngest` read
`LastRunAt('hae_rest')`, the newest `import_logs` row of any content. Monitor
9204 therefore reported `export received 49m0s ago` and stayed resolved through
33 hours in which nothing new was stored, against a threshold of 36 hours.

**Fix.** `fetchAll` asks for the day after the caller's end date, the boundary
both kinds of endpoint answer; `dayAfter` rejects a range end that is not
`YYYY-MM-DD` rather than sending it. `checkAppleIngest` reads
`LastStoredRunAt`, the newest delivery that wrote at least one row
(`metrics_inserted`, `workouts_inserted`, `sleep_sessions` or `sets_inserted`),
and the Settings text for the threshold says so. The missing days need no
backfill: the two-day overlap re-reads them on the next sync.

**Lesson.** A delivery is not a measurement. Both failures here left the
transport intact and changed only what it carried, so a rule that counts
requests reports the transport and misses the condition — it has to read what
was stored.

---

## 2026-09-20 — Every night's sleep stages were stored twice

**Symptoms.** The hypnogram drew a grey bar across the whole Awake lane, and
the night's first awakening was missing from it. The stage composition did not
add up: Deep 3:21 plus Core 8:16 plus REM 2:47 plus Awake 2:54 is 17:18, against
8:36 reported as time in bed, and the plot claimed 40 awakenings where the ring
reports about half. The figures above the composition — 7:13 asleep, 8:36 in
bed — were right, which is what kept this unnoticed.

Measured against the deployed instance on 2026-09-20 for the night of the 19th:
135 stage rows, 59 overlapping pairs, 17.27 hours of stages.

**Root cause.** Two defects, one visible only because of the other.

The ring reaches FreeReps twice. `internal/oura/sync.go` fetches the night from
the Oura API, where `parseSleepPhases` cuts `sleep_phase_5_min` into segments on
a 5-minute grid from `bedtime_start`. The Oura app also writes the same night
into HealthKit, from where the FreeReps iOS app forwards it:
`SyncService.swift:1293` iterates `HealthDataTypes.allCategoryTypes` with no way
to deselect one, and `internal/ingest/health/provider.go` turned those
`HKCategoryTypeIdentifierSleepAnalysis` samples into a second set of
`sleep_stages`. For that night, 38 rows on the Oura grid against 96 beside it.

Neither guard applied. The unique index on
`(start_time, end_time, stage, user_id)` compares interval bounds, and the two
deliveries differ by seconds to minutes. A source priority could not separate
them either, because both carry `source = 'Oura'` — the mapper sets the
constant, the iOS app passes `sourceDisplayName`, which for those samples is the
Oura app.

The grey bar is the second defect. HealthKit also reports an `In Bed` sample
spanning the night, which the Oura API never sends. `Hypnogram.tsx` computed its
lane as `Math.max(0, STAGE_LANES.indexOf(stage))`, so an unrecognised stage fell
into lane 0 — the Awake lane. Blocks render in array order and that sample sorts
directly after the night's first two awakenings, so it covered exactly those and
nothing after them.

**Fix.** Three changes. `Hypnogram.tsx` draws only stages that have a lane, so
an unrecognised one renders nowhere rather than across the first. The ingest
endpoint leaves sleep alone when the user's sleep resolves to a provider that
syncs on its own (`sleepClaimedBySync`); the category samples are still stored,
only the second extraction into `sleep_stages` is refused. Migration
`000032_dedupe_sleep_stages` removes the rows already written, identifying them
by the category sample with the same user and the same bounds, and only for
users whose priority names such a provider — a user without one keeps every row,
because for them this path is the only one. Rehearsed against the real night in
a scratch database: 248 rows to 64, `In Bed` gone, and 248 unchanged when the
priority names Apple Health.

**Lesson.** One provider is not one path. Oura arrives twice because its app
writes into HealthKit and a second client forwards that, and both deliveries
carry the provider's name — so the question "which source wins" cannot be
answered downstream. It has to be decided where the data enters.

---

## 2026-09-20 — The front page reported 32,361 steps for a day of 16,652

**Symptoms.** The front page and the metric page disagreed about the same day.
Measured against the deployed instance on 2026-09-20 for 2026-09-18:
`/api/v1/metrics/latest` returned 32,361.5 steps, `/api/v1/timeseries` with a
daily bucket returned 16,651.5. The front page series carried the same
inflation on every day with Oura data: 31,154 on 09-07, 32,575 on 09-08,
38,036 on 09-09. `/api/v1/metrics/stats` reported a third figure, an average of
925.1 over `count: 18`, which the metric page showed under the label "Mean".

**Root cause.** Two defects in `server/internal/storage`, both in the reduction
that runs before every aggregate.

`dedupCTE` numbered rows with `ROW_NUMBER() OVER (PARTITION BY
time_bucket('5 minutes', time) ORDER BY <source priority>)` and every caller
kept `rn = 1`. That expression decided two things at once: which source wins,
and that exactly one row per window survives. For a counter the second half
destroys the quantity — a sum over per-second samples becomes a sum over one of
them.

The source half failed where two sources use different reporting intervals.
Oura writes a day's steps as a single row at 12:00 while Apple Health writes
hourly blocks. Choosing per window kept the Oura total in its own window and
summed the Apple blocks around it: 16,651.5 − 176 + 15,886 = 32,361.5. The
metric page escaped this because `GetTimeSeries` resolves the `activity`
priority, where Apple Health leads; `GetDailySeries` resolved `_default`, where
Oura leads.

`GetMetricStats` used `AVG` for every metric, including counters, so its
headline figure answered what the average block of steps was.

Rows in the deployed database for 2026-09-18, which show both halves: 19
`step_count` rows, 18 hourly Apple blocks summing to 16,651.5 and one Oura row
of 15,886 at 12:00.

**Fix.** Source selection and row reduction were separated. The CTE now marks
every row of the winning source with `rn = 1` via `FIRST_VALUE(source)`, so the
predicate removes competing sources rather than competing samples. Cumulative
metrics resolve the source per day, everything else per 5-minute window, because
a daily choice would discard the windows the leading source missed — on the same
day Apple Health held 54 heart rate windows in which Oura had no row, covering
the morning strength session. Non-cumulative metrics average within the window
first and then across windows; `GetMetricStats` sums for cumulative metrics; and
multi-metric queries resolve each metric with its own category priority. See
`DECISIONS.md`, 2026-09-20.

**Lesson.** A reduction that both picks a source and drops rows will be correct
for one of the two jobs at a time. Where two sources report the same quantity at
different intervals, the choice belongs on the interval the quantity is stated
over, not on a fixed window.

---

## 2026-09-20 — The Withings integration delivered no measurement for 46 days

**Symptoms.** `import_logs` carried one `withings_sync` row every 30 minutes with
`status = error` and the message `getting token: refreshing token: decoding token
body: json: cannot unmarshal number into Go struct field tokenBody.userid of type
string`. Measured against the deployed instance on 2026-09-20 over the last
40,000 log rows: 2,204 `withings_sync` runs, of which 7 succeeded, all on
2026-08-05. The first of those wrote the deployment-day backfill (54 metrics
received, 54 inserted at 15:31Z); the six after it received one row and inserted
none. The first failure is 2026-08-05 18:48Z, 30 minutes after the last success,
and every run from then to the fix carried the same message.

The measurement series therefore stops on 2026-08-05 and resumes only with the
fix: `GET /api/v1/metrics?name=weight_body_mass` holds 56 rows with
`source = 'Withings'`, and the 29 of them dated after 2026-08-05 were all written
by the first run on the fixed binary.

The dashboard showed no error. The metrics stayed registered in
`metric_allowlist` and their series simply ended, which reads the same as "not
measured lately".

**Root cause.** `tokenBody.UserID` in `server/internal/withings/models.go` was
declared `string` with tag `json:"userid"`, matching the quoted form the Public
API guide documents. Withings sends the field as a bare number. `json.Unmarshal`
fails the whole struct on a type mismatch in one field, so `postToken` returned
an error instead of the token pair, and `GetValidToken` never reached the store.

Two properties turned one wrong field type into a 46-day outage:

- **The access token lives 3 hours**, so almost every sync cycle refreshes first.
  A refresh that cannot be decoded fails every subsequent sync, not only one.
- **The refresh token rotates, and the stored one survived anyway.** Withings
  issued a new pair on each of the 2,197 refreshes whose response FreeReps
  discarded. The expectation before the deploy was that the stored token was dead
  and the consent flow would have to run again; it was not. The rotation
  invalidates the previous token once the *new access token is first used*
  (`server/specs/withings-api.md`), and FreeReps never got that far, so the token
  stored on 2026-08-05 still refreshed 46 days later. Verified on the deployed
  instance: the run at 2026-09-20 10:46Z refreshed and wrote 29 measurements
  without any re-authorization.

**Fix.** `flexString` reads the field from either encoding, mirroring the
existing `flexBool` for `more` (commit `5bd5712`). `postToken` keeps returning an
error when the token pair is empty, so a genuinely malformed body still fails
loudly. `server/specs/withings-api.md` records both encodings and the date the
change was observed. `token_test.go` covers the numeric and the quoted form,
because a fix that swaps one for the other breaks the other direction.

**Verified** on the deployed binary `edge-5bd5712`: the 10:41Z run still carried
the decode error, the 10:46Z run succeeded with 30 metrics received and 29
inserted, and the weight series now ends 2026-09-20 06:42Z with
`source = 'Withings'`.

**Lesson.** A vendor field typed from the vendor's example is a single point of
failure for the whole response; read scalars that carry an identifier through a
decoder that accepts both encodings. The second lesson is about visibility: an
integration whose only failure signal is a row in `import_logs` can be down for
six weeks while every screen looks merely empty. The third is about the
diagnosis: the first reading of this outage claimed no measurement had ever
arrived, on the strength of a query sent with `?metric=` while the endpoint reads
`?name=` — an unknown parameter answers 400, and the empty result read as an
empty table.

---

## 2026-09-20 — One sport carried two workout names after the first Health Auto Export REST payload

**Symptoms.** After the first HAE REST export (25 workouts, 2026-09-12 to
2026-09-20), the workout list showed `Radfahren` and `Cycling` as separate types
for two rides on the same day. Filtering by type returned one of them; the
per-type grouping counted them apart.

An audit of all 2,851 stored workouts on 2026-09-20 found three names that are
not canonical: `Radfahren` (1 row), `Dance` (2), `yardwork` (1). `Tennis` (1) and
`Underwater Diving` (68) are their own canonical form and stay unchanged.

**Root cause.** `workoutNameMap` in `server/internal/ingest/workouts.go` held the
location-prefixed German names `Outdoor Radfahren` and `Innenräume Radfahren`,
but not the unprefixed `Radfahren` that HAE delivered. `NormalizeWorkoutName`
returns an unmapped name unchanged, by design — the map cannot know every
activity type — so the gap surfaces as a second name for a sport that already had
276 rows, not as an error.

`Dance` and `yardwork` are the same class: the canonical values `Dancing` and a
Yard Work entry existed only for the Oura spelling, or not at all.

**Fix.** Three map entries (`Radfahren` → `Cycling`, `Dance` → `Dancing`,
`yardwork` → `Yard Work`) plus migration
`000029_normalize_workout_names_round_two`, which renames the four stored rows.
`workouts_test.go` covers the three names and, as the counter-case, `Tennis` and
`Underwater Diving`, which must pass through unchanged.

**Lesson.** A source change that keeps the payload format can still change the
vocabulary inside it; after switching an ingest path, audit the distinct values of
every normalized column against the map rather than reading the first rows.

---

## 2026-08-10 — The Alpha Progression history was stored twice, offset by the Berlin UTC offset

**Symptoms.** `get_strength_summary` reported 378 working sets and 171,869 kg of
tonnage for January 2026 against 11 training days, and 22 `sessions` for the same
month. `get_strength_intensity` and `get_strength_volume` carried the same
inflation, and the RIR distribution was weighted toward the duplicated period.
No tool output marked anything as duplicated; the numbers were merely twice what
they should have been.

Measured against the deployed database on 2026-08-10: 117 of 280 stored sessions
existed twice, spanning 2025-03-18 to 2026-02-19 — the entire Alpha history up to
that date, not a window within it. The two copies of a session were identical in
`session_name` and in every set, warm-ups included; they differed only in
`session_date`, by 3600 s for sessions in CET and 7200 s for sessions in CEST.

**Root cause.** `parseSessionDate` in `server/internal/ingest/alpha/parser.go`
read the export's session time with `time.ParseInLocation(layout, s, time.Local)`.
The Alpha CSV carries a bare wall clock with no zone, so the instant it produced
was a property of the host running the import:

- an import running in `Europe/Berlin` read `2026-01-02 9:22` as `08:22Z`,
- the deployed container has no `/etc/localtime` and no `TZ`, so Go's
  `time.Local` is UTC there, and the same line read as `09:22Z`.

`session_date` is part of `workout_sets_source_natural_key` (migration
`000020_hevy.up.sql`), so the `ON CONFLICT DO NOTHING` in `InsertWorkoutSets`
compared two different keys and inserted rather than skipped. The insert order in
`workout_sets.id` shows it directly: ids 1–2936 hold the Berlin-read copy of all
117 sessions, ids 2991–5926 the UTC-read copy of the same 117, written by the
import logged at 2026-02-25 17:12 UTC (`import_logs` id 7, 2990 rows received,
2990 inserted — nothing conflicted, because every key had moved).

`time.Local` reached the parser through commit `39de7f1` (2026-02-21), which
replaced `time.Parse` with `time.ParseInLocation` to fix a genuine bug: read as
UTC, the wall clock was wrong by the local offset. The fix was correct in intent
and wrong in mechanism — it made the timestamp depend on the environment instead
of on a stated zone.

Cross-checked against the `workouts` table, which holds Apple Health workouts
with real zoned timestamps: on all 117 days the Berlin-read copy lands within
−20 to +16 minutes of that day's `Traditional Strength Training` start, and the
UTC-read copy 56 to 136 minutes after it. The earlier copy is the true one.

**Fix.** Three commits:

- The parser takes the zone as a parameter, supplied from
  `ingest.session_timezone` (see [`DECISIONS.md`](DECISIONS.md), 2026-08-10).
  Covered by `TestParseIsIndependentOfProcessTimezone` and, against a real
  database, by `TestReimportUnderADifferentProcessTimezoneIsIdempotent`
  (`-tags integration`).
- Migration `000027_dedupe_alpha_sessions` deleted the later copy of every
  session whose name and full set signature matched another copy of the same
  session: 117 sessions, 2936 set rows, of which 2367 working sets.
- The 46 sessions imported after 2026-02-21 needed a second step. They were
  written by the container alone, in UTC, so they had no duplicate — they were
  an hour or two late, and with the importer now reading Europe/Berlin a
  re-import would have inserted a corrected copy beside each of them.
  A script was written to move them by `UPDATE`. Before it ran, the export was
  re-imported, which did the same thing by a different route: it wrote the
  correct instants as 1067 new rows beside the stale ones, brought in one
  genuinely new session (2026-08-08) and left the 117 corrected sessions
  untouched, as they now conflict. Migration
  `000028_dedupe_alpha_sessions_after_reimport` then deleted the stale copies
  under the same content-matched rule as 000027 — 1067 set rows in 46 sessions,
  876 of them working sets — and the script was removed as obsolete.

  The window between the two steps is the lesson within the lesson: a
  half-corrected table is a table where the next import duplicates the
  uncorrected half. Migration 000027 and this correction should have shipped
  together.

`sessions` in `get_strength_summary` counted distinct session start times, which
is what it still does; the 22 for an 11-day January was the duplication showing
through, not a counting error. The field is now documented as such and a
`training_days` count sits beside it, because the two differ on any day holding
more than one session and nothing said which was being reported.

**Lesson.** A timestamp that is part of a natural key must be computed from the
file and stated configuration only — never from the process environment, which
differs between the machine that develops an importer and the container that
runs it.

---

## 2026-04-08 — Alpha Progression sets with "N+" notation were dropped or read as zero

**Symptoms.** Strength training sets imported from Alpha Progression CSV were
missing from the workout detail view, and some that did arrive carried an RIR of
0. The import reported success and no error; the set count was simply lower than
the CSV's.

**Root cause.** Alpha Progression writes `5+` for "at least 5" in the reps, RIR
and weight columns. Two code paths in `server/internal/ingest/alpha/parser.go`
handled it differently, and both silently:

- `parseEuropeanFloat` returned 0 for `5+`, so an RIR of "5 or more" became 0 —
  the value that means "to failure".
- The reps column was matched by a strict `\d+` regex, which rejected `5+`, and
  the rejection dropped the whole set line rather than the one field.

**Fix.** The trailing `+` is stripped before parsing, so the numeric value
survives in both paths (commit `bb929f1`). The commit adds cases to
`parser_test.go` covering the notation in each column.

**Lesson.** A parser that returns a zero value on a format it does not know
turns a format gap into wrong data; return an error or normalize explicitly.

---

## 2026-03-26 — Sleep durations of 22 hours and more after the Oura integration

**Symptoms.** Sleep sessions showed impossible durations — 22 hours and above —
for nights where both Oura and Health Auto Export had delivered data. Correct
Oura sessions had been present earlier and were overwritten.

**Root cause.** The sleep session backfill in `server/internal/storage/sleep.go`
read all rows in `sleep_stages` for a date regardless of source and summed their
durations. With two sources reporting the same night, every stage was counted
twice. The backfill then wrote the result over the session a direct source had
already produced.

**Fix.** Commit `731b001`. The backfill uses `ON CONFLICT DO NOTHING`, so it only
creates sessions for dates that have none, and never overwrites a direct source.
Oura sync writes `sleep_analysis` metrics itself, so the dashboard chart no
longer depends on the backfill running.

**Lesson.** A derivation that aggregates across sources needs a source filter at
the point it reads, not a priority applied afterwards.

---

## 2026-03-25 — Respiratory rate 60× too high, SpO2 shown as 0.94 for one source and 94.5 for the other

**Symptoms.** After the Oura integration, respiratory rate read around 900 per
minute. SpO2 rendered as `0.94` for Apple Health values and `94.5` for Oura
values in the same chart.

**Root cause.** Two unit mismatches in `server/internal/oura/mapper.go`:

- The Oura OpenAPI specification documents `average_breath` as breaths per
  second. It is breaths per minute. The ingest applied a ×60 conversion the data
  did not need.
- Apple Health stores SpO2 as a fraction (`0.94`), Oura as a percentage
  (`94.5`). Neither was normalized, so both landed in the same metric.

**Fix.** Commit `5554cd9` removes the ×60 conversion, normalizes Oura SpO2 to a
fraction on ingest, and adds a `display_multiplier` of ×100 for presentation.
Migration `000019_fix_spo2_multiplier` corrects the rows already stored.

**Lesson.** Vendor documentation is a claim about units, not evidence — compare
a known value against the vendor's own app before trusting a conversion factor,
and store one unit per metric across all sources.
