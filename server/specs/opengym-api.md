# openGym HTTP API — Wire Format Spec and Ingest

openGym (`github.com/DuarteSantos8/openGym`, AGPL-3.0) is a self-hosted
workout logger under evaluation as the replacement for Hevy. This document has
two parts:

1. **Wire format** — what the openGym server returns, measured against the
   operator's instance on 2026-11-05 and read from upstream `api/openapi.yaml`
   at commit `8fb98b16` (2026-11-04). The instance runs image tag `1.4.4`
   (homelab `docker/stacks/opengym/compose.yaml`). Every route listed under
   *Endpoints FreeReps uses* was called against it.
2. **Ingest** — how FreeReps stores openGym sessions in `workout_sets`
   (`server/internal/opengym/`). The design choices are in `DECISIONS.md`,
   2026-11-05; the open questions are at the end.

The instance is `https://gym.meltrocker.net` (homelab
`configuration/docker-stacks/stacks/opengym.yml`), on marty. FreeReps reaches
it as described under *Network reach* in Part 2.

---

## Part 1 — Wire format

### Overview

- **No resource API.** openGym has no `/workouts` or `/routines` route. A
  profile is one JSON document (the *state*), read and written whole through
  `/api/data`. Everything FreeReps needs is inside that document.
- **Auth:** `Authorization: Bearer <token>`. The token is the same
  HMAC-signed session token a browser cookie carries — a **full sign-in to the
  profile, including write access**. openGym has no read-only or scoped token.
- **No delta feed and no webhook FreeReps can use.** See *Alternatives* in
  Part 2.
- Body limit on writes is 5 MiB; FreeReps does not write.

### Endpoints FreeReps uses

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/api/pair/redeem` | Turns a pairing code into a Bearer token (connect flow) |
| `GET` | `/api/me` | Validates a token; renews it past half its lifetime |
| `GET` | `/api/data/rev` | Current revision of the state; the cheap change check |
| `GET` | `/api/data` | The whole state, including every workout |

### `POST /api/pair/redeem`

No auth. Body `{ "code": "<8 chars>" }`, answers `{ "token": "<78 chars>" }`.

- The code is minted in a **browser** signed in to the profile: Settings →
  Account → *Pair the mobile app*, confirmed with the passkey. The iOS app
  cannot mint one on an instance without password sign-in (`PASSWORD_LOGIN`
  is unset on this instance).
- **The code is 8 characters from `ABCDEFGHJKLMNPQRSTUVWXYZ23456789`, no
  dashes**, one-shot, valid 5 minutes, held in server memory. The same Account
  page offers *Add a device*, whose code looks like `XXXX-XXXX-XXXX`; that is a
  device link that registers a passkey and is refused here with
  `400 {"error":"invalid or expired code","code":"pair-invalid"}`. This mix-up
  happened on 2026-11-05.
- Pairing again yields a new token and does **not** revoke the old one. Only
  *Sign out everywhere* (or `POST /api/logout/all`) revokes tokens, and it
  revokes all of them — every paired phone and every other consumer.

### `GET /api/me`

```json
{ "user": { "id": "SghntmYPzElOHPCQ", "name": "Linus", "admin": true, "owner": true } }
```

Past half of `SESSION_DAYS` (unset on this instance, so 90 days) the response
also carries `"token": "<renewed token>"`, to be used from then on. A token
that is not renewed expires after `SESSION_DAYS`; every authenticated route then
answers `401 {"error":"not signed in"}`.

### `GET /api/data/rev`

```json
{ "rev": 3, "wid": "cec6ae102e8461ff" }
```

`rev` increases with every accepted write (app sync, API write, server-side
stamping). `wid` identifies the stored document; it changes when the data
directory is restored from a backup and `rev` may then repeat. A change check
compares the pair, not `rev` alone.

### `GET /api/data`

```json
{ "state": { "...": "the profile" }, "rev": 3 }
```

Measured size on 2026-11-05: 51,286 bytes for 29 workouts and 3 routines. The
state has about 90 top-level keys; FreeReps reads these:

| Key | Use |
|---|---|
| `workouts[]` | Logged sessions, chronological |
| `unit` | `kg` or `lb`; weights are stored in this unit |
| `customEx[]` | The user's own exercises: `id`, name in `n`, equipment in `eq`, body part in `bp` (upstream `openapi.yaml`, `CustomExercise`) |
| `effort` | `rpe`, `rir` or `none` — the scale the app asks for (unset here; every set carried `rpe`) |
| `deleted` | Record of removals for merging devices (absent here). FreeReps does not read it: a deleted workout is removed from `workouts[]` |
| `_rev`, `_wid` | Mirror of the revision and write id |

`exUnit` (per-exercise unit, `{ "<exercise id>": { "unit": "kg" | "lb" | null, "_ts": … } }`)
changes only how the app shows and takes input for an exercise. Every weight in
the state stays in `unit` (upstream `frontend/src/lib/ex-unit.js`, header
comment). FreeReps does not read it. The app treats `unit` values other than
`lb`, including an absent `unit`, as `kg` (`profileUnit` in the same file).

### Workout object

Measured on the instance (the Upper session of 2026-11-05, shortened to two exercises and two sets each):

```json
{
  "id": "iwfp6ie5n94d",
  "d": "2026-11-05",
  "start": 1793857222000,
  "end": 1793863673000,
  "routineId": null,
  "name": "Upper · Block 2026-10",
  "entries": [
    { "id": "0025", "topW": 102.5, "sets": [
      { "w": 20,    "r": 10, "done": true, "phase": "warmup", "rpe": 6 },
      { "w": 102.5, "r": 6,  "done": true, "rpe": 8.5 }
    ] },
    { "id": "0652", "topW": 5, "sets": [
      { "w": 0, "r": 6, "done": true, "phase": "warmup", "rpe": 7 },
      { "w": 5, "r": 8, "done": true, "rpe": 8 }
    ] }
  ],
  "prs": [],
  "vol": 7293.5
}
```

| Field | Meaning |
|---|---|
| `id` | Stable workout id. Observed prefixes: `iw` for workouts imported from Hevy, `mw` for workouts logged through openGym's MCP server, `w_<epoch ms>` in upstream examples for app-logged ones. The prefixes are an observation, not documented API. |
| `d` | The day as the phone saw it (local date), `YYYY-MM-DD` |
| `start`, `end` | Epoch **milliseconds**. `end` can be absent on an unfinished or paper-logged workout. |
| `routineId` | openGym routine id the session was started from, or `null`. The Hevy import sets `null` for every workout. |
| `name` | Session name |
| `entries[].id` | **Catalogue id** (`"0025"`, four or five digits) or a custom exercise id. No exercise name is in the workout. |
| `entries[].sets[]` | Planned and performed sets, in order |
| `prs`, `vol`, `topW` | Values the app derives; FreeReps ignores them |

Set fields:

| Field | Meaning | Observed |
|---|---|---|
| `w` | Weight in `unit`. On a body-weight exercise it is **added** load; on an assisted exercise (`lever assisted …`) it is **assistance** and is subtracted from body weight by the app. | yes |
| `r` | Reps. On a per-side exercise, the total across both sides. | yes |
| `done` | `false` is a planned set that was not performed. | `true` only |
| `phase: "warmup"` | Warm-up set | yes |
| `rpe` / `rir` | Effort on the scale the profile uses, half-point steps | `rpe` on all 650 sets |
| `failure: true` | Set taken to failure | no |
| `sec`, `min`, `speed`, `incline` | Timed and cardio sets | no |
| `sides: { left, right }` | Per-side reps | no |
| `type: "restpause"`, `clusters[]` | Rest-pause set | no |

The unobserved rows come from upstream's webhook payload documentation
(`docs/INTEGRATIONS.md`) and the app's own source; check them against a captured
state before relying on them.

### Exercise names are not served

The catalogue (6,554 entries, `catalogue/exercises/<id>.json` upstream: name,
equipment, target muscle, secondary muscles) is bundled into the web app's
JavaScript. No route serves it. A consumer that needs names or muscles for a
catalogue id carries its own table. Custom exercises are in `customEx[]` with
their names.

### Hevy's catalogue and openGym's catalogue

openGym's Hevy importer resolves Hevy `exercise_template_id`s through a
generated table, `frontend/src/lib/hevy-id-map.js`. It contains at least one
wrong row: `023943F1` *Chin Up (Weighted)* maps to `0572`
*lever assisted chin-up*, which inverts the meaning of the weight. The
operator's 7 affected sessions were moved to `1326` *chin-up* by an API write
on 2026-11-05. Do not derive FreeReps' mapping from that file without
reviewing every row.

---

## Part 2 — Ingest

**Status: implemented 2026-11-05.** Decisions in `DECISIONS.md`, 2026-11-05:
*openGym is read through its HTTP API*, *openGym exercises are mapped by their
id*, *The openGym sync rewrites a workout only when its mapped rows change*.
Code: migrations `000040`, `000041`; `server/internal/opengym/` (client,
mapper, sync, pairing); `server/internal/server/handlers_opengym.go`;
Settings → openGym.

### Scope

In: strength sessions from `workouts[]` into `workout_sets`, with source name
`openGym`, so every existing training aggregate and MCP tool covers them.

Out, each for a stated reason:

- **Routines and plans.** FreeReps holds no plan (`DECISIONS.md`, 2026-08-04,
  *FreeReps stays the system of record; planning lives outside it*).
- **Writes to openGym.** Same decision.
- **Body weight from openGym.** FreeReps already has it from Withings and Apple
  Health; a third source would need a priority decision for nothing gained.
- **openGym's AI coach.** It runs inside openGym and is to be disabled there.

### Transport: poll the state

Per cycle (`opengym.sync_interval`, 30 minutes like Hevy):

1. `GET /api/me` — fails fast on a revoked token, and stores a renewed token
   when the response carries one.
2. `GET /api/data/rev` — if `(rev, wid)` equals the stored pair, the cycle ends.
3. `GET /api/data` — reconcile the workouts:
   - every workout that starts on or after `sync_from` is mapped; when the
     SHA-256 of its mapped rows differs from the hash in `opengym_workouts`,
     its sets are deleted by `external_id` and inserted again, so an edited set
     replaces the stored one and a removed set disappears;
   - every workout in `opengym_workouts` that is no longer in the state has its
     sets deleted. A full state makes this exact, which Hevy's event feed needs
     a `deleted` event for. A state without workouts next to stored ones is
     refused rather than read as a deletion of all of them;
   - a workout the mapper refuses (`ErrUnsupported`) or that fails to store is
     recorded as an error; the others are stored.
4. Store `(rev, wid)` if the cycle had no error, rebuild `strength_tonnage`
   from `sync_from` when anything was written, and write the `import_logs` row
   (`source = opengym_sync`, status `error` if any workout failed).

**Alternatives.**

- *openGym webhook `workout.finished`.* Rejected: openGym sends no event for
  an edited or deleted workout, so a correction made in the app would never
  arrive, and it delivers only events of the last seven days after the webhook
  was added, so it carries no backfill. An inbound endpoint would also need an
  authentication of its own next to the Tailscale identity (`DECISIONS.md`,
  2026-03-15). The 2026-08-04 decision rejected Hevy's webhook on comparable
  grounds.
- *Read `state-<uid>.json` from openGym's data directory*, as openGym's own
  MCP server does. Rejected; see `DECISIONS.md`, 2026-11-05, *openGym is read
  through its HTTP API, not from its data directory*.

### Credentials

Migration `000041`: `opengym_credentials` (user, `base_url`, `token`,
`sync_from` as a day in the session timezone), `opengym_sync_state` (`rev`,
`wid`), `opengym_workouts` (workout id, hash of its mapped rows). The schema is
in `database-schema.md`.

Connect flow in Settings → openGym: instance URL (https, host only), pairing
code and `sync_from`. The server checks all three before it redeems the code,
because a code is valid once: a code with dashes is refused with the hint about
the device-link code, and a `sync_from` on or before the day of the last Hevy
session is refused (see *Overlap with the Hevy history*). It then redeems the
code, validates the token with `/api/me` and stores it. The token never reaches
the browser. Disconnecting deletes the stored token; it stays valid in openGym
until it expires or *Sign out everywhere* revokes it.

| Route | Method | Purpose |
|---|---|---|
| `/api/v1/opengym/status` | GET | Connection, `sync_from`, last change read; when unconfigured the earliest allowed `sync_from` |
| `/api/v1/opengym/pair` | PUT | `{base_url, code, sync_from}` — redeem and store |
| `/api/v1/opengym/sync` | POST | Start a cycle now |
| `/api/v1/opengym/disconnect` | DELETE | Remove the connection; stored sets stay |

FreeReps gets **its own pairing**, separate from the token the strength-coach
project holds (`claude/opengym-token` in setec), so each consumer can be
re-paired without the other. Revocation is not per consumer (see
`/api/pair/redeem` above).

The token can write the whole profile. FreeReps only reads; the Hevy key it
stores today has the same reach (`hevy-api.md`, *Overview*).

### Mapping a set to a `workout_sets` row

| Column | Value |
|---|---|
| `source` | `openGym` |
| `external_id` | `workout.id` |
| `routine_id` | `workout.routineId` or `''` |
| `session_name` | `workout.name` |
| `session_date` | `workout.start` (epoch ms → instant). Without `start`: `d` at the configured timezone, as the Alpha ingest does (`DECISIONS.md`, 2026-08-10). |
| `session_end`, `session_duration` | from `end`, when present |
| `exercise_number` | entry index + 1 |
| `exercise_name` | title of the mapped catalog entry; for a custom exercise its `customEx` `n`; for an unmapped id `openGym exercise <id>` |
| `source_exercise_id` | `entries[].id` — the key into `exercise_name_map` |
| `exercise_template_id` | `''` — the column holds Hevy template ids, and the join in `QueryWorkoutSets` treats any non-empty value as one |
| `set_number` | index among the entry's performed sets, + 1 |
| `is_warmup`, `set_type` | `phase == "warmup"` → `true`, `warmup`; `failure` → `failure`; else `normal` |
| `weight_kg` | `w`, multiplied by 0.45359237 when `unit` is `lb` (openGym's own factor, `units.js` `LB_KG`) |
| `is_bodyweight_plus` | `true` when the mapped catalog entry's `exercise_type` is `bodyweight_weighted`; for a custom exercise when `eq` is `body weight` or `weighted` |
| `reps` | `r` |
| `rpe` / `rir` | as delivered, never converted (`DECISIONS.md`, 2026-03-25) |
| `duration_sec`, `distance_m` | timed and cardio sets, once observed |

Sets with `done: false` are not stored. **Assisted exercises** need a decision:
their `w` is not load, and storing it as `weight_kg` adds it to tonnage. Until
then the mapper refuses a workout containing an exercise mapped onto a
`bodyweight_assisted` entry, and likewise a performed set with an unobserved
shape (`type`, `sides`, `clusters`, `sec`, `min`, `speed`, `incline`).

### Muscle attribution: map onto the stored Hevy catalogue

Muscle groups exist only in `exercise_templates`, in Hevy's 20-value
vocabulary. The proposal maps each openGym catalogue id onto a Hevy template, so
volume per muscle group continues across the source change in one vocabulary.
`exercise_templates` stays in the database after the Hevy sync stops — no code
path or migration deletes from it (checked 2026-11-05) — so every template
stored today remains usable as a mapping target.

The map is `exercise_name_map`, keyed by `(source, source_key)`, where
`source_key` is the openGym id (`DECISIONS.md`, 2026-11-05, *openGym exercises
are mapped by their id*). Seed in migration `000040`, for the 23 exercises in
the operator's history plus `0572`, reviewed by hand on 2026-11-05 against the
Hevy rows FreeReps holds for the same sessions:

| openGym id | openGym name | Hevy template | Hevy title |
|---|---|---|---|
| `0025` | barbell bench press | `79D0BB3A` | Bench Press (Barbell) |
| `0031` | barbell curl | `A5AC6449` | Bicep Curl (Barbell) |
| `0047` | barbell incline bench press | `50DFDFAB` | Incline Bench Press (Barbell) |
| `0150` | cable bar lateral pulldown | `6A6C31A5` | Lat Pulldown (Cable) |
| `0192` | cable one arm lateral raise | `DE68C825` | Single Arm Lateral Raise (Cable) |
| `0201` | cable pushdown | `93A552C6` | Triceps Pushdown |
| `0277` | decline crunch | `B2398CD1` | Decline Crunch (Weighted) |
| `0334` | dumbbell lateral raise | `422B08F1` | Lateral Raise (Dumbbell) |
| `0383` | dumbbell reverse fly | `B582299E` | Chest Supported Reverse Fly (Dumbbell) |
| `0410` | dumbbell single leg split squat | `B5D3A742` | Bulgarian Split Squat (Dumbbell) |
| `0447` | ez barbell curl | `01A35BF9` | EZ Bar Biceps Curl |
| `0472` | hanging leg raise | `F8356514` | Hanging Leg Raise |
| `0572` | lever assisted chin-up | `D23C609B` | Chin Up (Assisted) — `bodyweight_assisted`, refused by the mapper until assisted exercises are decided |
| `0586` | lever lying leg curl | `B8127AD1` | Lying Leg Curl (Machine) |
| `0602` | lever seated reverse fly | `D8281C62` | Rear Delt Reverse Fly (Machine) |
| `0605` | lever standing calf raise | `E05C2C38` | Standing Calf Raise (Machine) |
| `0652` | pull-up | `729237D1` | Pull Up (Weighted) |
| `0739` | sled 45° leg press | `C7973E0E` | Leg Press (Machine) |
| `1326` | chin-up | `023943F1` | Chin Up (Weighted) — 26 of 29 chin-up sets in the history carry load; *Chin Up* (`29083183`) has the same muscle groups |
| `1454` | lever seated shoulder press | `9237BAD1` | Seated Shoulder Press (Machine) — performed on a Smith machine; the Alpha map uses `B09A1304` for the same movement |
| `1755` | weighted tricep dips | `10347BAC` | Triceps Dip (Weighted) |
| `2611` | lever horizontal one leg press | `0EB695C9` | Leg Press Horizontal (Machine) |
| `2661` | cable seated row with v bar | `0393F233` | Seated Cable Row - V Grip (Cable) |
| `5035` | weighted hyperextension | `091737FA` | Back Extension (Weighted Hyperextension) |

Anything not in the map counts in `unmapped_sets` until a row is added; that
figure is how a new exercise becomes visible. `ListUnmappedExerciseNames`
returns the `source_key` a new row needs.

The table is data written for FreeReps, not code taken from openGym (AGPL-3.0;
FreeReps is MIT) — the same distinction as the HealthLog references in
`DECISIONS.md`.

### Overlap with the Hevy history

openGym's history contains every Hevy workout since 2026-08-11 (imported with
`iw` ids), and FreeReps already holds those as `source = Hevy`. Ingesting both
doubles tonnage and set counts, because the training aggregates do not filter
on source (roadmap row *Source deduplication in the training aggregates*).

- `sync_from` is the guard, as for Hevy: the first day logged in openGym, after
  the last Hevy session. The pairing handler refuses a `sync_from` on or before
  the day of the newest stored Hevy session (session timezone) and defaults to
  the day after it (`opengym.ResolveSyncFrom`). The `iw` prefix is not a guard;
  it is undocumented.
- **A trial period logged in both apps** is the trigger of that roadmap row. Either
  one app logs during the trial, or the deduplication lands first.
- Once openGym is the logger, the Hevy credentials are removed and the Hevy
  rows stay. `checkSourceStale` stays silent for a user whose newest run is
  older than the threshold (`DECISIONS.md`, 2026-09-27), so removing the key
  produces no alert.

### Alerts

- **9209 `freereps - opengym sync`:** consecutive failed runs (HTTP error,
  `401` after a revocation, unparseable state, a refused workout) reach
  `checkSource` like every polled source.
- **9210 `freereps - opengym data`:** `opengym_sync` stored no workout for 14
  days, counting `workouts_inserted` or `sets_inserted` above 0 — the reasoning
  of the Hevy row (id 9208) applies unchanged. A workout counts as inserted only
  when its mapped rows changed, so a weigh-in does not reset the condition.

### Network reach

The client (`internal/opengym/client.go`) uses Go's default dialer, not the
tsnet node, so a request leaves the container through smokey's network stack
and arrives at marty as smokey (`tag:homelab`, `tag:ci-target`,
`tag:apps-own-web`). marty carries `tag:homelab` next to `tag:apps-web`, and the
homelab grant `tag:homelab` → `tag:homelab` on `tcp:443`
(`infrastructure/tailnet-policy/policy.hujson`) admits the connection. The
`tag:apps-web` grant is not involved. Measured on 2026-11-05 from inside
`freereps-app-1`: `gym.meltrocker.net` resolves to marty (`100.119.115.91`) and
`GET /api/health` answers `{"ok":true,…}`.

If that grant is narrowed, the sync fails. A blocked peer is absent from the
netmap, so the run fails with a DNS or dial error, not with a refused
connection.

### Tests

- Mapper: `internal/opengym/mapper_test.go` on `testdata/state-cut.json`, a
  fixture cut from the operator's state.
- Reconciliation against a fake store: `sync_test.go` — an edited set, a
  deleted workout, an unchanged `(rev, wid)` that skips the fetch, a changed
  `wid` at the same `rev`, a new `rev` without workout change, a refused
  workout, an empty state, token renewal and a rejected token.
- Against the scratch database: `sync_integration_test.go` (a second sync with
  the same workouts writes nothing; a deletion removes the sets) and
  `storage/exercise_map_integration_test.go` (the map join).
- Pairing against the recorded answers, including `pair-invalid`:
  `pair_test.go`.

### Open questions

1. Assisted exercises: skip `weight_kg`, store it negative, or flag the set.
   The mapper refuses them until this is decided.
2. Shapes of timed, cardio, per-side and rest-pause sets — capture from a
   state that contains them. The mapper refuses the set shapes until then.
   (`deleted` needs no shape: deleting a workout in the app removes it from
   `workouts[]`, upstream `frontend/src/sheets.jsx` `deleteWorkout`.)
3. Whether `d` or `start` defines the session day for a workout logged on paper
   or moved to another date in the app ("move it to the right date"). The
   mapper uses `start` and falls back to `d`.

The provider label (`DECISIONS.md`, 2026-09-21) needed no change:
`models.RecordedBy` returns the source name. The workout detail page needed
`openGym` in its list of sources whose sessions exist only in `workout_sets`
(`web/src/pages/WorkoutDetailPage.tsx`).
