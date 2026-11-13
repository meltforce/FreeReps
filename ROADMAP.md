# Roadmap

Open work, one line per item. A closed item is deleted. The rows with their
full notes as of 2026-11-13 are frozen in `archive/ROADMAP-2026-11-13.md`.

## Dashboard

- The HAE ingest discards the device name, so iPhone and Watch share one source — Health Auto Export is used as a delivery path again, or the iOS app delivers metric rows per device
- `get_workouts` in the MCP misses the strength sessions the workout list merges — Alpha Progression is used again, or another provider delivers sessions that have no workout row
- Workouts, sleep stages and category samples do not record which client delivered them — A question about which client delivered a workout or a sleep stage, or both clients delivering one
- The metric table draws its sparklines in the frame tone
- Persist the selected time range per screen
- Manual entry for muscle mass
- Lag correlation over the full series, server-side — Windows beyond one year
- The Oura sync repeats a full backfill window for `vo2_max` on every cycle

## Training data

- Pair FreeReps with openGym and retire the Hevy key — The operator switches the logger to openGym (at the earliest at the block revision of 2026-11-22)
- openGym: decide how assisted exercises are stored — An assisted exercise is logged in openGym — the sync then fails with `ErrUnsupported` on alert 9209
- openGym: capture the unobserved set shapes — A timed, cardio, per-side or rest-pause set is logged — the sync then fails with `ErrUnsupported` on alert 9209
- openGym: `d` or `start` for a workout logged on paper or moved to another day — A workout logged on paper or moved in the app
- Tonnage ignores body weight — The exercise mix changes, so that tonnage is compared across periods that log body-weight work differently
- Planning project
- Backfill the Alpha history into Hevy
- Source deduplication in the training aggregates — Two sources cover one period
- `Bench Press` names two different exercises
- 2026-05-02 holds nine sets, all flagged as warm-ups
- Detail view for sessions without a workout row
