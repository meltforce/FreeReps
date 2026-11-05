# openGym HTTP API — Wire Format Spec and Ingest Proposal

openGym (`github.com/DuarteSantos8/openGym`, AGPL-3.0) is a self-hosted
workout logger under evaluation as the replacement for Hevy. This document has
two parts:

1. **Wire format** — what the openGym server returns, measured against the
   operator's instance on 2026-11-05 and read from upstream `api/openapi.yaml`
   at commit `8fb98b16` (2026-11-04). The instance runs image tag `1.4.4`
   (homelab `docker/stacks/opengym/compose.yaml`). Every route listed under
   *Endpoints FreeReps uses* was called against it.
2. **Ingest proposal** — how FreeReps would store openGym sessions in
   `workout_sets`. Nothing in that part is decided. Each design choice moves to
   `DECISIONS.md` when it is taken, and this section shrinks accordingly.

The instance is `https://gym.meltrocker.net` (homelab
`configuration/docker-stacks/stacks/opengym.yml`), on marty, reachable through
the `tag:apps-web` 443 grant.

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
| `exUnit` | Per-exercise unit override (empty on this instance; shape unverified) |
| `customEx[]` | The user's own exercises: id, name, muscles |
| `effort` | `rpe`, `rir` or `none` — the scale the app asks for (unset here; every set carried `rpe`) |
| `deleted` | Record of removals, written once something is deleted (absent here; shape unverified) |
| `_rev`, `_wid` | Mirror of the revision and write id |

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

## Part 2 — Ingest proposal

**Status: proposal, 2026-11-05.** Not implemented, nothing decided.

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
   - every workout with `start >= sync_from` is written with
     delete-then-insert per `external_id`, the same shape as the Hevy ingest
     (`server/internal/hevy/sync.go`, `storeWorkout`), so an edited set
     replaces the stored one and a removed set disappears;
   - every stored `openGym` `external_id` whose workout is no longer in the
     state is deleted. A full state makes this exact, which Hevy's event feed
     needs a `deleted` event for.
4. Store `(rev, wid)` and write the `import_logs` row (`source = opengym_sync`).

**Alternatives.**

- *openGym webhook `workout.finished`.* Rejected: openGym sends no event for
  an edited or deleted workout, so a correction made in the app would never
  arrive, and it delivers only events of the last seven days after the webhook
  was added, so it carries no backfill. An inbound endpoint would also need an
  authentication of its own next to the Tailscale identity (`DECISIONS.md`,
  2026-03-15). The 2026-08-04 decision rejected Hevy's webhook on comparable
  grounds.
- *Read `./data/state-<uid>.json` from disk*, as openGym's own MCP server does.
  Rejected: FreeReps runs on smokey, openGym on marty; it would need a mount
  across hosts and bypass the application's revision handling.

### Credentials

A table in the shape of `hevy_credentials`:

```sql
CREATE TABLE opengym_credentials (
    user_id    INTEGER     NOT NULL PRIMARY KEY REFERENCES users(id),
    base_url   TEXT        NOT NULL,   -- https only, like openGym's own MCP client
    token      TEXT        NOT NULL,
    sync_from  DATE        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- plus opengym_sync_state (user_id, rev, wid, updated_at)
```

Connect flow in the settings screen: base URL and pairing code; the server
redeems the code, validates the token with `/api/me`, and stores it. This
replaces a token pasted by hand and keeps the token out of the browser. A
failed redeem shows the `pair-invalid` hint about the dashed device-link code.

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
| `exercise_name` | catalogue name from the map below, or the `customEx` name |
| `exercise_template_id` | `''` — the column holds Hevy template ids, and the join in `QueryWorkoutSets` treats any non-empty value as one |
| `set_number` | index among the entry's performed sets, + 1 |
| `is_warmup`, `set_type` | `phase == "warmup"` → `true`, `warmup`; `failure` → `failure`; else `normal` |
| `weight_kg` | `w`, converted when the unit is `lb` |
| `is_bodyweight_plus` | `true` when the catalogue equipment is `body weight` or `weighted` (pull-up, chin-up, dips, hyperextension) |
| `reps` | `r` |
| `rpe` / `rir` | as delivered, never converted (`DECISIONS.md`, 2026-03-25) |
| `duration_sec`, `distance_m` | timed and cardio sets, once observed |

Sets with `done: false` are not stored. **Assisted exercises** need a decision
before the first one is logged: their `w` is not load, and storing it as
`weight_kg` adds it to tonnage.

### Muscle attribution: map onto the stored Hevy catalogue

Muscle groups exist only in `exercise_templates`, in Hevy's 20-value
vocabulary. The proposal maps each openGym catalogue id onto a Hevy template, so
volume per muscle group continues across the source change in one vocabulary.
`exercise_templates` stays in the database after the Hevy sync stops — no code
path or migration deletes from it (checked 2026-11-05) — so every template
stored today remains usable as a mapping target.

`exercise_name_map` is keyed by `(source, exercise_name)`. **The key should be
the openGym id, not the name.** The state carries no names, so for catalogue
exercises the name would be FreeReps' own table value acting as its own key,
and custom exercises keep their id while the user renames them in the app. The
minimal change is a column `workout_sets.source_exercise_id` and a map keyed on
`(source, source_exercise_id)`, joined where `exercise_name_map` is joined
today: `storage/workout_sets.go:130`, `storage/training_volume.go:55`,
`storage/exercise_catalog.go:113`. The choice between that and reusing
`exercise_name_map` is the implementer's, and goes into `DECISIONS.md`.

Seed for the 24 exercises in the operator's history, reviewed by hand on
2026-11-05 against the Hevy titles FreeReps holds for the same sessions:

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
| `0586` | lever lying leg curl | `B8127AD1` | Lying Leg Curl (Machine) |
| `0602` | lever seated reverse fly | `D8281C62` | Rear Delt Reverse Fly (Machine) |
| `0605` | lever standing calf raise | `E05C2C38` | Standing Calf Raise (Machine) |
| `0652` | pull-up | `729237D1` | Pull Up (Weighted) |
| `0739` | sled 45° leg press | `C7973E0E` | Leg Press (Machine) |
| `1326` | chin-up | `29083183` | Chin Up |
| `1454` | lever seated shoulder press | `9237BAD1` | Seated Shoulder Press (Machine) — performed on a Smith machine; the Alpha map uses `B09A1304` for the same movement |
| `1755` | weighted tricep dips | `10347BAC` | Triceps Dip (Weighted) |
| `2611` | lever horizontal one leg press | `0EB695C9` | Leg Press Horizontal (Machine) |
| `2661` | cable seated row with v bar | `0393F233` | Seated Cable Row - V Grip (Cable) |
| `5035` | weighted hyperextension | `091737FA` | Back Extension (Weighted Hyperextension) |

`0572` *lever assisted chin-up* is deliberately absent. Anything not in the
map counts in `unmapped_sets` until a row is added; that figure is how a new
exercise becomes visible.

The table is data written for FreeReps, not code taken from openGym (AGPL-3.0;
FreeReps is MIT) — the same distinction as the HealthLog references in
`DECISIONS.md`.

### Overlap with the Hevy history

openGym's history contains every Hevy workout since 2026-08-11 (imported with
`iw` ids), and FreeReps already holds those as `source = Hevy`. Ingesting both
doubles tonnage and set counts, because the training aggregates do not filter
on source (roadmap row *Source deduplication in the training aggregates*).

- `sync_from` is the guard, as for Hevy: the first day logged in openGym, after
  the last Hevy session. The `iw` prefix is not a guard; it is undocumented.
- **A trial period logged in both apps** is the trigger of that roadmap row. Either
  one app logs during the trial, or the deduplication lands first.
- Once openGym is the logger, the Hevy credentials are removed and the Hevy
  rows stay. `checkSourceStale` stays silent for a user whose newest run is
  older than the threshold (`DECISIONS.md`, 2026-09-27), so removing the key
  produces no alert.

### Alerts

- A failed run (HTTP error, `401` after a revocation, unparseable state) reaches
  `checkSource` like every polled source.
- A stale condition with its own id, `opengym_sync`, 14 days, counting
  `workouts_inserted` or `sets_inserted` above 0 — the reasoning of the Hevy row
  (id 9208) applies unchanged.

### Prerequisite outside this repo: network reach

The grant onto `tag:apps-web` 443 (homelab
`infrastructure/tailnet-policy/policy.hujson`) admits `group:family`,
`tag:juno`, `tag:magnum`, `tag:claude` and `tag:home-assistant`. FreeReps runs on
smokey with a tsnet node `freereps`; neither identity is in that list.
Which of the two originates the outbound request depends on whether the HTTP
client dials through tsnet or through the host — the policy file's note 3 on
tsnet twins covers why that matters. **Measure it, then add the grant in the
homelab repo**, with its entry there. A blocked node is absent from the netmap,
so the failure presents as a DNS error, not as a refused connection.

### Tests

- Mapper unit tests on a fixture cut from a real state (`start` absent, a
  warm-up, a body-weight set, a `done: false` set, a custom exercise).
- Reconciliation: an edited set, a deleted workout, an unchanged `(rev, wid)`
  that skips the fetch, a changed `wid` at the same `rev`.
- Integration test against the scratch database: a second sync with the same
  state writes nothing; the unique constraint holds.
- The connect flow against a recorded `pair-invalid` answer.

### Open questions

1. Map key: new column and map table, or reuse `exercise_name_map`.
2. Assisted exercises: skip `weight_kg`, store it negative, or flag the set.
3. Shapes of `deleted`, `exUnit`, timed, cardio, per-side and rest-pause sets —
   capture from a state that contains them.
4. Whether `d` or `start` defines the session day for a workout logged on paper
   or moved to another date in the app ("move it to the right date").
5. Whether the web UI's provider label (`DECISIONS.md`, 2026-09-21) shows
   `openGym` for these sessions without further change.
